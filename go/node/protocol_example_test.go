package node

import core "dappco.re/go"

func ExampleProtocolError_Error() {

	_ = (*ProtocolError).Error

	core.Println("ProtocolError.Error")

	// Output: ProtocolError.Error

}

func ExampleResponseHandler_ValidateResponse() {

	_ = (*ResponseHandler).ValidateResponse

	core.Println("ResponseHandler.ValidateResponse")

	// Output: ResponseHandler.ValidateResponse

}

func ExampleResponseHandler_ParseResponse() {

	_ = (*ResponseHandler).ParseResponse

	core.Println("ResponseHandler.ParseResponse")

	// Output: ResponseHandler.ParseResponse

}

func ExampleValidateResponse() {

	_ = ValidateResponse

	core.Println("ValidateResponse")

	// Output: ValidateResponse

}

func ExampleParseResponse() {

	_ = ParseResponse

	core.Println("ParseResponse")

	// Output: ParseResponse

}

func ExampleIsProtocolError() {

	_ = IsProtocolError

	core.Println("IsProtocolError")

	// Output: IsProtocolError

}

func ExampleGetProtocolErrorCode() {

	_ = GetProtocolErrorCode

	core.Println("GetProtocolErrorCode")

	// Output: GetProtocolErrorCode

}
