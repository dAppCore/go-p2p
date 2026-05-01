package api

import core "dappco.re/go"

func ExampleNewProvider() {

	_ = NewProvider

	core.Println("NewProvider")

	// Output: NewProvider

}

func ExampleP2PProvider_Name() {

	_ = (*P2PProvider).Name

	core.Println("P2PProvider.Name")

	// Output: P2PProvider.Name

}

func ExampleP2PProvider_BasePath() {

	_ = (*P2PProvider).BasePath

	core.Println("P2PProvider.BasePath")

	// Output: P2PProvider.BasePath

}

func ExampleP2PProvider_RegisterRoutes() {

	_ = (*P2PProvider).RegisterRoutes

	core.Println("P2PProvider.RegisterRoutes")

	// Output: P2PProvider.RegisterRoutes

}

func ExampleP2PProvider_Describe() {

	_ = (*P2PProvider).Describe

	core.Println("P2PProvider.Describe")

	// Output: P2PProvider.Describe

}

func ExampleP2PProvider_Channels() {

	_ = (*P2PProvider).Channels

	core.Println("P2PProvider.Channels")

	// Output: P2PProvider.Channels

}
