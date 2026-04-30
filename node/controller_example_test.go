package node

import core "dappco.re/go"

func ExampleNewController() {

	_ = NewController

	core.Println("NewController")

	// Output: NewController

}

func ExampleController_GetRemoteStats() {

	_ = (*Controller).GetRemoteStats

	core.Println("Controller.GetRemoteStats")

	// Output: Controller.GetRemoteStats

}

func ExampleController_StartRemoteMiner() {

	_ = (*Controller).StartRemoteMiner

	core.Println("Controller.StartRemoteMiner")

	// Output: Controller.StartRemoteMiner

}

func ExampleController_StopRemoteMiner() {

	_ = (*Controller).StopRemoteMiner

	core.Println("Controller.StopRemoteMiner")

	// Output: Controller.StopRemoteMiner

}

func ExampleController_GetRemoteLogs() {

	_ = (*Controller).GetRemoteLogs

	core.Println("Controller.GetRemoteLogs")

	// Output: Controller.GetRemoteLogs

}

func ExampleController_GetRemoteLogsSince() {

	_ = (*Controller).GetRemoteLogsSince

	core.Println("Controller.GetRemoteLogsSince")

	// Output: Controller.GetRemoteLogsSince

}

func ExampleController_GetAllStats() {

	_ = (*Controller).GetAllStats

	core.Println("Controller.GetAllStats")

	// Output: Controller.GetAllStats

}

func ExampleController_PingPeer() {

	_ = (*Controller).PingPeer

	core.Println("Controller.PingPeer")

	// Output: Controller.PingPeer

}

func ExampleController_ConnectToPeer() {

	_ = (*Controller).ConnectToPeer

	core.Println("Controller.ConnectToPeer")

	// Output: Controller.ConnectToPeer

}

func ExampleController_DisconnectFromPeer() {

	_ = (*Controller).DisconnectFromPeer

	core.Println("Controller.DisconnectFromPeer")

	// Output: Controller.DisconnectFromPeer

}
