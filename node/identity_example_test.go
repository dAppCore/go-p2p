package node

import core "dappco.re/go"

func ExampleGenerateChallenge() {

	_ = GenerateChallenge

	core.Println("GenerateChallenge")

	// Output: GenerateChallenge

}

func ExampleSignChallenge() {

	_ = SignChallenge

	core.Println("SignChallenge")

	// Output: SignChallenge

}

func ExampleVerifyChallenge() {

	_ = VerifyChallenge

	core.Println("VerifyChallenge")

	// Output: VerifyChallenge

}

func ExampleNewNodeManager() {

	_ = NewNodeManager

	core.Println("NewNodeManager")

	// Output: NewNodeManager

}

func ExampleNewNodeManagerWithPaths() {

	_ = NewNodeManagerWithPaths

	core.Println("NewNodeManagerWithPaths")

	// Output: NewNodeManagerWithPaths

}

func ExampleLoadOrCreateIdentity() {

	_ = LoadOrCreateIdentity

	core.Println("LoadOrCreateIdentity")

	// Output: LoadOrCreateIdentity

}

func ExampleLoadOrCreateIdentityWithPaths() {

	_ = LoadOrCreateIdentityWithPaths

	core.Println("LoadOrCreateIdentityWithPaths")

	// Output: LoadOrCreateIdentityWithPaths

}

func ExampleNodeManager_HasIdentity() {

	_ = (*NodeManager).HasIdentity

	core.Println("NodeManager.HasIdentity")

	// Output: NodeManager.HasIdentity

}

func ExampleNodeManager_GetIdentity() {

	_ = (*NodeManager).GetIdentity

	core.Println("NodeManager.GetIdentity")

	// Output: NodeManager.GetIdentity

}

func ExampleNodeManager_GenerateIdentity() {

	_ = (*NodeManager).GenerateIdentity

	core.Println("NodeManager.GenerateIdentity")

	// Output: NodeManager.GenerateIdentity

}

func ExampleNodeManager_DeriveSharedSecret() {

	_ = (*NodeManager).DeriveSharedSecret

	core.Println("NodeManager.DeriveSharedSecret")

	// Output: NodeManager.DeriveSharedSecret

}

func ExampleNodeManager_Delete() {

	_ = (*NodeManager).Delete

	core.Println("NodeManager.Delete")

	// Output: NodeManager.Delete

}
