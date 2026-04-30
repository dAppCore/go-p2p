package node

import core "dappco.re/go"

func ExampleNewWorker() {

	_ = NewWorker

	core.Println("NewWorker")

	// Output: NewWorker

}

func ExampleWorker_SetMinerManager() {

	_ = (*Worker).SetMinerManager

	core.Println("Worker.SetMinerManager")

	// Output: Worker.SetMinerManager

}

func ExampleWorker_SetProfileManager() {

	_ = (*Worker).SetProfileManager

	core.Println("Worker.SetProfileManager")

	// Output: Worker.SetProfileManager

}

func ExampleWorker_HandleMessage() {

	_ = (*Worker).HandleMessage

	core.Println("Worker.HandleMessage")

	// Output: Worker.HandleMessage

}

func ExampleWorker_RegisterWithTransport() {

	_ = (*Worker).RegisterWithTransport

	core.Println("Worker.RegisterWithTransport")

	// Output: Worker.RegisterWithTransport

}
