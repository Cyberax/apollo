package utils

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"strings"
)

func KvListToMap(kvList []string) (map[string]string, error) {
	env := map[string]string{}
	for _, k := range kvList {
		splits := strings.SplitN(k, "=", 2)
		if len(splits) != 2 {
			return map[string]string{}, fmt.Errorf("format is not 'k=v': %s", k)
		}
		env[splits[0]] = splits[1]
	}
	return env, nil
}


func CheckRequiredFlags(flags *pflag.FlagSet) error {
	requiredError := false
	flagName := ""

	flags.VisitAll(func(flag *pflag.Flag) {
		requiredAnnotation := flag.Annotations[cobra.BashCompOneRequiredFlag]
		if len(requiredAnnotation) == 0 {
			return
		}

		flagRequired := requiredAnnotation[0] == "true"

		if flagRequired && !flag.Changed {
			requiredError = true
			flagName = flag.Name
		}
	})

	if requiredError {
		return errors.New("Required flag `" + flagName + "` has not been set")
	}

	return nil
}


func GetFlagS(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(err.Error())
	}
	return val
}

func GetFlagB(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(err.Error())
	}
	return val
}

func GetFlagI(cmd *cobra.Command, name string) int64 {
	val, err := cmd.Flags().GetInt64(name)
	if err != nil {
		panic(err.Error())
	}
	return val
}
