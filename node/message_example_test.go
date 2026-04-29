package node

import core "dappco.re/go"

func ExampleRawMessage_MarshalJSON() {

	_ = RawMessage.MarshalJSON

	core.Println("RawMessage.MarshalJSON")

	// Output: RawMessage.MarshalJSON

}

func ExampleRawMessage_UnmarshalJSON() {

	_ = (*RawMessage).UnmarshalJSON

	core.Println("RawMessage.UnmarshalJSON")

	// Output: RawMessage.UnmarshalJSON

}

func ExampleIsProtocolVersionSupported() {

	_ = IsProtocolVersionSupported

	core.Println("IsProtocolVersionSupported")

	// Output: IsProtocolVersionSupported

}

func ExampleNewMessage() {

	_ = NewMessage

	core.Println("NewMessage")

	// Output: NewMessage

}

func ExampleMessage_Reply() {

	_ = (*Message).Reply

	core.Println("Message.Reply")

	// Output: Message.Reply

}

func ExampleMessage_ParsePayload() {

	_ = (*Message).ParsePayload

	core.Println("Message.ParsePayload")

	// Output: Message.ParsePayload

}

func ExampleNewErrorMessage() {

	_ = NewErrorMessage

	core.Println("NewErrorMessage")

	// Output: NewErrorMessage

}
