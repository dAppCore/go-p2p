package contentbus

import core "dappco.re/go"

func ExampleNewController() {

	_ = NewController

	core.Println("NewController")

	// Output: NewController

}

func ExampleWithNodeManager() {

	_ = WithNodeManager

	core.Println("WithNodeManager")

	// Output: WithNodeManager

}

func ExampleWithPeerRegistry() {

	_ = WithPeerRegistry

	core.Println("WithPeerRegistry")

	// Output: WithPeerRegistry

}

func ExampleWithTransport() {

	_ = WithTransport

	core.Println("WithTransport")

	// Output: WithTransport

}

func ExampleWithTransportConfig() {

	_ = WithTransportConfig

	core.Println("WithTransportConfig")

	// Output: WithTransportConfig

}

func ExampleWithChannelBuffer() {

	_ = WithChannelBuffer

	core.Println("WithChannelBuffer")

	// Output: WithChannelBuffer

}

func ExampleWithStartTransport() {

	_ = WithStartTransport

	core.Println("WithStartTransport")

	// Output: WithStartTransport

}
