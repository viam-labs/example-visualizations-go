package main

import (
	exampleviz "exampleviz"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

func main() {
	module.ModularMain(
		resource.APIModel{API: worldstatestore.API, Model: exampleviz.Model},
		resource.APIModel{API: worldstatestore.API, Model: exampleviz.VisualizerModel},
		resource.APIModel{API: generic.API, Model: exampleviz.DriverModel},
	)
}
