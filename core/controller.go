package core

import (
	log "github.com/sirupsen/logrus"
)

// Controller managers all the submodules.
type Controller struct {
	txsCh         chan NewTxsEvent
}

// NewController creates a new controller object
func NewController() *Controller {
	return &Controller{}
}

// Start starts the controller
func (c *Controller) Start() {
	log.Debug("controller started")
}

// Stop stops the controller
func (c *Controller) Stop() {
	log.Debug("controller stopped")
}

