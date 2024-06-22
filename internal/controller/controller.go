package controller

import (
	"fmt"
	"time"

	"github.com/Erik142/vault-op-autounseal/internal/config"
	"github.com/Erik142/vault-op-autounseal/internal/vault"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AutoUnsealController struct {
	ClientSet *kubernetes.Clientset
	Config    *config.Config
}

func New(restConfig *rest.Config) (*AutoUnsealController, error) {
	client, err := client.New(restConfig, client.Options{})

	if err != nil {
		panic(fmt.Errorf("Could not create Kubernetes client: %v\n", err))
	}

	clientset := kubernetes.NewForConfigOrDie(restConfig)

	err = config.Init(client)

	if err != nil {
		panic(fmt.Errorf("Could not create application configuration: %v\n", err))
	}

	c, err := config.Get()

	if err != nil {
		panic(fmt.Errorf("Could not retrieve application configuration: %v\n", err))
	}

	return &AutoUnsealController{Config: c, ClientSet: clientset}, nil
}

func (self *AutoUnsealController) Reconcile() error {
	for true {
		vault, err := vault.New(self.ClientSet)

		if err != nil {
			log.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err = vault.Init(); err != nil {
			log.Error(err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err = vault.Unseal(); err != nil {
			log.Error(err)
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}
