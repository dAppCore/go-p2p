package ueps

import core "dappco.re/go"

func ExampleNewBuilder() {

	_ = NewBuilder

	core.Println("NewBuilder")

	// Output: NewBuilder

}

func ExamplePacketBuilder_MarshalAndSign() {

	_ = (*PacketBuilder).MarshalAndSign

	core.Println("PacketBuilder.MarshalAndSign")

	// Output: PacketBuilder.MarshalAndSign

}
