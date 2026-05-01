package node

import core "dappco.re/go"

func ExampleNewDispatcher() {

	_ = NewDispatcher

	core.Println("NewDispatcher")

	// Output: NewDispatcher

}

func ExampleDispatcher_RegisterHandler() {

	_ = (*Dispatcher).RegisterHandler

	core.Println("Dispatcher.RegisterHandler")

	// Output: Dispatcher.RegisterHandler

}

func ExampleDispatcher_Handlers() {

	_ = (*Dispatcher).Handlers

	core.Println("Dispatcher.Handlers")

	// Output: Dispatcher.Handlers

}

func ExampleDispatcher_Dispatch() {

	_ = (*Dispatcher).Dispatch

	core.Println("Dispatcher.Dispatch")

	// Output: Dispatcher.Dispatch

}
