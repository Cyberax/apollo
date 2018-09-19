package aposerver

//go:generate go run ../vendor/github.com/rakyll/statik/statik.go -src=../swagger-ui/dist

//noinspection GoInvalidPackageImport
import (
	"apollo/data"
	"apollo/proto/gen/restapi/operations/login"
	"apollo/proto/gen/restapi/operations/node"
	"apollo/proto/gen/restapi/operations/queue"
	"apollo/proto/gen/restapi/operations/task"

	"apollo/aposerver/statik"
	// The squashed Swagger UI
	_ "apollo/aposerver/statik"
	"apollo/proto/gen/restapi"
	"apollo/proto/gen/restapi/operations"
	"apollo/utils"
	"github.com/go-openapi/errors"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jessevdk/go-flags"
	"github.com/rakyll/statik/fs"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	"strings"
	"time"
)

func uiMiddleware(ctx *ServerContext, handler http.Handler) http.Handler {
	statikFS, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	staticFiles := http.StripPrefix("/public/", http.FileServer(statikFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Shortcut helpers for swagger-ui
		if r.URL.Path == "/" || r.URL.Path == "" {
			http.Redirect(w, r, "/public/", http.StatusMovedPermanently);
			return
		}
		if r.URL.Path == "/public/" || r.URL.Path == "/public/index.html" {
			http.ServeContent(w, r, "index.html", time.Now(),
				strings.NewReader(statik.ApplicationIndex))
			return
		}
		if strings.Index(r.URL.Path, "/public") == 0 {
			staticFiles.ServeHTTP(w, r)
			return
		}

		// Add the logger to the request
		r = prepareContext(r)
		// Keep track of the request ID
		reqId := r.Header.Get("X-Request-Id")
		if reqId != "" {
			w.Header().Add("X-Request-Id", reqId)
		}
		// Delegate to the next handler
		handler.ServeHTTP(w, r)
	})
}

func WireUpHandlers(ctx *ServerContext, api *operations.ApolloAPI) {
	// Login
	api.LoginPostSigv4LoginHandler = login.PostSigv4LoginHandlerFunc(
		func(params login.PostSigv4LoginParams) middleware.Responder {
			lp := LoginProcessor {
				ctx: params.HTTPRequest.Context(),
				aws: ctx.AwsConfig,
				store: ctx.TokenStore,
				serverCert: ctx.TlsManager.OurCert,
				whitelistedAccounts: ctx.WhitelistedAccounts,
				params: params,
			}
			return lp.Enact()
		})

	api.LoginGetNodeTokenHandler = login.GetNodeTokenHandlerFunc(
		func(params login.GetNodeTokenParams, principal interface{}) middleware.Responder {
			lp := GetNodeTokenProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.TokenStore,
				serverCert: ctx.TlsManager.OurCert,
				principal: principal.(data.AuthToken),
				params: params,
			}
			return lp.Enact()
		})

	// Pingy-pongy!
	api.LoginGetPingHandler = login.GetPingHandlerFunc(
		func(params login.GetPingParams, principal interface{}) middleware.Responder {
			return login.NewGetPingOK()
		})

	// Tasks
	api.TaskPutTaskHandler = task.PutTaskHandlerFunc(
		func(params task.PutTaskParams, principal interface{}) middleware.Responder {
			tp := TaskSubmitProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.TaskStore,
				queueStore: ctx.QueueStore,
				kvStore: ctx.KvStore,
				principal: principal.(data.AuthToken),
				params: params,
			}
			return tp.Enact()
		})

	api.TaskGetTaskListHandler = task.GetTaskListHandlerFunc(
		func(params task.GetTaskListParams, principal interface{}) middleware.Responder {
			lp := ListTasksProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.TaskStore,
				params: params,
			}
			return lp.Enact()
	})

	// Queues
	api.QueueGetQueueListHandler = queue.GetQueueListHandlerFunc(
		func(params queue.GetQueueListParams, principal interface{}) middleware.Responder {
			lq := ListQueueProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.QueueStore,
				params: params,
			}
			return lq.Enact()
		})

	api.QueuePutQueueHandler = queue.PutQueueHandlerFunc(
		func(params queue.PutQueueParams, principal interface{}) middleware.Responder {
			pq := PutQueueProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.QueueStore,
				params: params,
			}
			return pq.Enact()
		})

	api.QueueDeleteQueueHandler = queue.DeleteQueueHandlerFunc(
		func(params queue.DeleteQueueParams, principal interface{}) middleware.Responder {
			dq := DeleteQueueProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.QueueStore,
				taskStore: ctx.TaskStore,
				params: params,
			}
			return dq.Enact()
		})

	// Nodes
	api.NodePutUnmanagedNodeHandler = node.PutUnmanagedNodeHandlerFunc(
		func(params node.PutUnmanagedNodeParams, principal interface{}) middleware.Responder {
			dq := PutUnmanagedNodeProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.NodeStore,
				queueStore: ctx.QueueStore,
				params: params,
				principal: principal.(data.AuthToken),
			}
			return dq.Enact()
		})

	api.NodeGetNodeListHandler = node.GetNodeListHandlerFunc(
		func(params node.GetNodeListParams, principal interface{}) middleware.Responder {
			ln := ListNodesProcessor{
				ctx: params.HTTPRequest.Context(),
				store: ctx.NodeStore,
				params: params,
			}
			return ln.Enact()
		})
}

// Create a contextual logger with the request ID field set
func prepareContext(r *http.Request) *http.Request {
	reqId := r.Header.Get("X-Request-Id")
	context := utils.SaveLoggerToContext(r.Context(), logrus.StandardLogger())
	if reqId != "" {
		utils.AddLoggerFields(context, logrus.Fields{"RequestID": reqId})
	}
	context = utils.SaveReqIdToContext(context, reqId)
	return r.WithContext(context)
}

func serverError(request *http.Request, e error, writer http.ResponseWriter) {
	utils.CL(request.Context()).Infof("Failed to run operation: %s", e.Error())
	errors.ServeError(writer, request, e)
}

func RunServer(ctx *ServerContext) error {
	// load embedded swagger file
	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		log.Fatalln(err)
	}

	// create new service API
	api := operations.NewApolloAPI(swaggerSpec)
	// Hook up error response handler. TODO: add metrics
	api.ServeError = func(writer http.ResponseWriter, request *http.Request, e error) {
		serverError(request, e, writer)
	}

	server := restapi.NewServer(api)
	defer func() {
		_ = server.Shutdown()
	}()

	// Set up the middleware (Swagger UI, auth, web interface routing)
	api.Middleware = func(builder middleware.Builder) http.Handler {
		return uiMiddleware(ctx, api.Context().APIHandler(builder))
	}

	api.APIKeyAuthAuth = func(token string) (interface{}, error) {
		// Authenticate the request
		authToken, ok := ctx.TokenStore.GetTokenByKey(token)
		expireTime := authToken.Expires
		if !ok || (expireTime != data.NeverExpires && expireTime.ToTime().Before(time.Now())) {
			return nil, errors.Unauthenticated("https")
		}
		return authToken, nil
	}

	// set the port this service will be run on
	server.EnabledListeners = []string{}
	server.EnabledListeners = append(server.EnabledListeners, "https")
	server.TLSPort = ctx.TlsManager.TLSPort
	server.TLSHost = ctx.TlsManager.TLSHost
	server.TLSCertificate = flags.Filename(ctx.TlsManager.TLSCertFile)
	server.TLSCertificateKey = flags.Filename(ctx.TlsManager.TLSKeyFile)

	// Wire up handlers
	WireUpHandlers(ctx, api)

	err = server.Listen()
	if err != nil {
		return err
	}

	// Start the background reapers
	stopChannel := RunReapers(ctx)
	defer func() {
		stopChannel <- true
	}()

	// serve API
	if err = server.Serve(); err != nil {
		return err
	}
	return nil
}
