package node

import core "dappco.re/go"

func ExampleDefaultTransportConfig() {

	_ = DefaultTransportConfig

	core.Println("DefaultTransportConfig")

	// Output: DefaultTransportConfig

}

func ExampleNewMessageDeduplicator() {

	_ = NewMessageDeduplicator

	core.Println("NewMessageDeduplicator")

	// Output: NewMessageDeduplicator

}

func ExampleMessageDeduplicator_IsDuplicate() {

	_ = (*MessageDeduplicator).IsDuplicate

	core.Println("MessageDeduplicator.IsDuplicate")

	// Output: MessageDeduplicator.IsDuplicate

}

func ExampleMessageDeduplicator_Mark() {

	_ = (*MessageDeduplicator).Mark

	core.Println("MessageDeduplicator.Mark")

	// Output: MessageDeduplicator.Mark

}

func ExampleMessageDeduplicator_Cleanup() {

	_ = (*MessageDeduplicator).Cleanup

	core.Println("MessageDeduplicator.Cleanup")

	// Output: MessageDeduplicator.Cleanup

}

func ExampleNewPeerRateLimiter() {

	_ = NewPeerRateLimiter

	core.Println("NewPeerRateLimiter")

	// Output: NewPeerRateLimiter

}

func ExamplePeerRateLimiter_Allow() {

	_ = (*PeerRateLimiter).Allow

	core.Println("PeerRateLimiter.Allow")

	// Output: PeerRateLimiter.Allow

}

func ExampleNewTransport() {

	_ = NewTransport

	core.Println("NewTransport")

	// Output: NewTransport

}

func ExampleTransport_Start() {

	_ = (*Transport).Start

	core.Println("Transport.Start")

	// Output: Transport.Start

}

func ExampleTransport_Stop() {

	_ = (*Transport).Stop

	core.Println("Transport.Stop")

	// Output: Transport.Stop

}

func ExampleTransport_OnMessage() {

	_ = (*Transport).OnMessage

	core.Println("Transport.OnMessage")

	// Output: Transport.OnMessage

}

func ExampleTransport_Connect() {

	_ = (*Transport).Connect

	core.Println("Transport.Connect")

	// Output: Transport.Connect

}

func ExampleTransport_Send() {

	_ = (*Transport).Send

	core.Println("Transport.Send")

	// Output: Transport.Send

}

func ExampleTransport_Connections() {

	_ = (*Transport).Connections

	core.Println("Transport.Connections")

	// Output: Transport.Connections

}

func ExampleTransport_Broadcast() {

	_ = (*Transport).Broadcast

	core.Println("Transport.Broadcast")

	// Output: Transport.Broadcast

}

func ExampleTransport_GetConnection() {

	_ = (*Transport).GetConnection

	core.Println("Transport.GetConnection")

	// Output: Transport.GetConnection

}

func ExamplePeerConnection_Send() {

	_ = (*PeerConnection).Send

	core.Println("PeerConnection.Send")

	// Output: PeerConnection.Send

}

func ExamplePeerConnection_Close() {

	_ = (*PeerConnection).Close

	core.Println("PeerConnection.Close")

	// Output: PeerConnection.Close

}

func ExamplePeerConnection_GracefulClose() {

	_ = (*PeerConnection).GracefulClose

	core.Println("PeerConnection.GracefulClose")

	// Output: PeerConnection.GracefulClose

}

func ExampleTransport_ConnectedPeers() {

	_ = (*Transport).ConnectedPeers

	core.Println("Transport.ConnectedPeers")

	// Output: Transport.ConnectedPeers

}
