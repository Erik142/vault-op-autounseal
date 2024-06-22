package main

import (
	"flag"
	"path/filepath"

	"github.com/Erik142/vault-op-autounseal/internal/controller"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var debug *bool
var kubeconfig *string
var appconfig *string

func parseFlags() {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	appconfig = flag.String("config", "", "absolute path to the application config file")
	debug = flag.Bool("debug", false, "(optional) print debug messages")

	flag.Parse()
}

func getRestConfig() (*rest.Config, error) {
	// use the current context in kubeconfig
	restConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)

	if err != nil {
		restConfig, err = rest.InClusterConfig()
	}

	return restConfig, err
}

func main() {
	parseFlags()
	log.SetFormatter(&log.JSONFormatter{})

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	var restConfig *rest.Config
	var c *controller.AutoUnsealController
	var err error

	if restConfig, err = getRestConfig(); err != nil {
		panic(err)
	}

	if c, err = controller.New(restConfig, *appconfig); err != nil {
		panic(err)
	}

	if err := c.Reconcile(); err != nil {
		panic(err)
	}
}
