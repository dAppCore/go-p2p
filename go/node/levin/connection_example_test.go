package levin

import core "dappco.re/go"

func ExampleNewConnection() {

	_ = NewConnection

	core.Println("NewConnection")

	// Output: NewConnection

}

func ExampleConnection_WritePacket() {

	_ = (*Connection).WritePacket

	core.Println("Connection.WritePacket")

	// Output: Connection.WritePacket

}

func ExampleConnection_WriteResponse() {

	_ = (*Connection).WriteResponse

	core.Println("Connection.WriteResponse")

	// Output: Connection.WriteResponse

}

func ExampleConnection_ReadPacket() {

	_ = (*Connection).ReadPacket

	core.Println("Connection.ReadPacket")

	// Output: Connection.ReadPacket

}

func ExampleConnection_Close() {

	_ = (*Connection).Close

	core.Println("Connection.Close")

	// Output: Connection.Close

}

func ExampleConnection_RemoteAddr() {

	_ = (*Connection).RemoteAddr

	core.Println("Connection.RemoteAddr")

	// Output: Connection.RemoteAddr

}
