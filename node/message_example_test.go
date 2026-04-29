package node

import core "dappco.re/go"

func ExampleRawMessage_MarshalRawJSON() {

	_ = RawMessage.MarshalRawJSON

	core.Println("RawMessage.MarshalRawJSON")

	// Output: RawMessage.MarshalRawJSON

}

func ExampleRawMessage_UnmarshalRawJSON() {

	_ = (*RawMessage).UnmarshalRawJSON

	core.Println("RawMessage.UnmarshalRawJSON")

	// Output: RawMessage.UnmarshalRawJSON

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
