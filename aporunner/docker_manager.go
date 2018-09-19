package aporunner

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

type DockerContext struct {
	QueueName string

	Client *client.Client
	AuthToken string
	Repo, Login, Password string
}

func NewDockerContext(name string) (*DockerContext, error) {
	envClient, e := client.NewEnvClient()
	if e != nil {
		return nil, e
	}
	return &DockerContext{
		QueueName: name,
		Client:    envClient,
	}, nil
}

func (d *DockerContext) DoLogin(repo string, login string, pass string) error {
	logrus.Infof("Logging into %s", repo)
	ctx := context.Background()

	body, err := d.Client.RegistryLogin(ctx, types.AuthConfig{
		ServerAddress: repo,
		Username: login,
		Password: pass,
	})
	if err != nil {
		return err
	}

	if body.Status == "" {
		return fmt.Errorf("failed to get the token from the Docker repository")
	}
	d.AuthToken = body.IdentityToken
	d.Repo = repo
	d.Login = login
	d.Password = pass
	return nil
}
