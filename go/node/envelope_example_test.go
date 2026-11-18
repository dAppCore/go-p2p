package node

import core "github.com/dappcore/go"

func ExampleEnvelope_VerifySignature() {

	_ = Envelope.VerifySignature

	core.Println("Envelope.VerifySignature")

	// Output: Envelope.VerifySignature

}
