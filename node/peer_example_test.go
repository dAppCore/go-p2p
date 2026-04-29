package node

import core "dappco.re/go"

func ExampleNewPeerRegistry() {

	_ = NewPeerRegistry

	core.Println("NewPeerRegistry")

	// Output: NewPeerRegistry

}

func ExampleNewPeerRegistryWithPath() {

	_ = NewPeerRegistryWithPath

	core.Println("NewPeerRegistryWithPath")

	// Output: NewPeerRegistryWithPath

}

func ExamplePeerRegistry_SetAuthMode() {

	_ = (*PeerRegistry).SetAuthMode

	core.Println("PeerRegistry.SetAuthMode")

	// Output: PeerRegistry.SetAuthMode

}

func ExamplePeerRegistry_GetAuthMode() {

	_ = (*PeerRegistry).GetAuthMode

	core.Println("PeerRegistry.GetAuthMode")

	// Output: PeerRegistry.GetAuthMode

}

func ExamplePeerRegistry_AllowPublicKey() {

	_ = (*PeerRegistry).AllowPublicKey

	core.Println("PeerRegistry.AllowPublicKey")

	// Output: PeerRegistry.AllowPublicKey

}

func ExamplePeerRegistry_RevokePublicKey() {

	_ = (*PeerRegistry).RevokePublicKey

	core.Println("PeerRegistry.RevokePublicKey")

	// Output: PeerRegistry.RevokePublicKey

}

func ExamplePeerRegistry_IsPublicKeyAllowed() {

	_ = (*PeerRegistry).IsPublicKeyAllowed

	core.Println("PeerRegistry.IsPublicKeyAllowed")

	// Output: PeerRegistry.IsPublicKeyAllowed

}

func ExamplePeerRegistry_IsPeerAllowed() {

	_ = (*PeerRegistry).IsPeerAllowed

	core.Println("PeerRegistry.IsPeerAllowed")

	// Output: PeerRegistry.IsPeerAllowed

}

func ExamplePeerRegistry_ListAllowedPublicKeys() {

	_ = (*PeerRegistry).ListAllowedPublicKeys

	core.Println("PeerRegistry.ListAllowedPublicKeys")

	// Output: PeerRegistry.ListAllowedPublicKeys

}

func ExamplePeerRegistry_AllowedPublicKeys() {

	_ = (*PeerRegistry).AllowedPublicKeys

	core.Println("PeerRegistry.AllowedPublicKeys")

	// Output: PeerRegistry.AllowedPublicKeys

}

func ExamplePeerRegistry_AddPeer() {

	_ = (*PeerRegistry).AddPeer

	core.Println("PeerRegistry.AddPeer")

	// Output: PeerRegistry.AddPeer

}

func ExamplePeerRegistry_UpdatePeer() {

	_ = (*PeerRegistry).UpdatePeer

	core.Println("PeerRegistry.UpdatePeer")

	// Output: PeerRegistry.UpdatePeer

}

func ExamplePeerRegistry_RemovePeer() {

	_ = (*PeerRegistry).RemovePeer

	core.Println("PeerRegistry.RemovePeer")

	// Output: PeerRegistry.RemovePeer

}

func ExamplePeerRegistry_GetPeer() {

	_ = (*PeerRegistry).GetPeer

	core.Println("PeerRegistry.GetPeer")

	// Output: PeerRegistry.GetPeer

}

func ExamplePeerRegistry_ListPeers() {

	_ = (*PeerRegistry).ListPeers

	core.Println("PeerRegistry.ListPeers")

	// Output: PeerRegistry.ListPeers

}

func ExamplePeerRegistry_Peers() {

	_ = (*PeerRegistry).Peers

	core.Println("PeerRegistry.Peers")

	// Output: PeerRegistry.Peers

}

func ExamplePeerRegistry_UpdateMetrics() {

	_ = (*PeerRegistry).UpdateMetrics

	core.Println("PeerRegistry.UpdateMetrics")

	// Output: PeerRegistry.UpdateMetrics

}

func ExamplePeerRegistry_UpdateScore() {

	_ = (*PeerRegistry).UpdateScore

	core.Println("PeerRegistry.UpdateScore")

	// Output: PeerRegistry.UpdateScore

}

func ExamplePeerRegistry_SetConnected() {

	_ = (*PeerRegistry).SetConnected

	core.Println("PeerRegistry.SetConnected")

	// Output: PeerRegistry.SetConnected

}

func ExamplePeerRegistry_MarkSeen() {

	_ = (*PeerRegistry).MarkSeen

	core.Println("PeerRegistry.MarkSeen")

	// Output: PeerRegistry.MarkSeen

}

func ExamplePeerRegistry_RecordSuccess() {

	_ = (*PeerRegistry).RecordSuccess

	core.Println("PeerRegistry.RecordSuccess")

	// Output: PeerRegistry.RecordSuccess

}

func ExamplePeerRegistry_RecordFailure() {

	_ = (*PeerRegistry).RecordFailure

	core.Println("PeerRegistry.RecordFailure")

	// Output: PeerRegistry.RecordFailure

}

func ExamplePeerRegistry_RecordTimeout() {

	_ = (*PeerRegistry).RecordTimeout

	core.Println("PeerRegistry.RecordTimeout")

	// Output: PeerRegistry.RecordTimeout

}

func ExamplePeerRegistry_GetPeersByScore() {

	_ = (*PeerRegistry).GetPeersByScore

	core.Println("PeerRegistry.GetPeersByScore")

	// Output: PeerRegistry.GetPeersByScore

}

func ExamplePeerRegistry_PeersByScore() {

	_ = (*PeerRegistry).PeersByScore

	core.Println("PeerRegistry.PeersByScore")

	// Output: PeerRegistry.PeersByScore

}

func ExamplePeerRegistry_SelectOptimalPeer() {

	_ = (*PeerRegistry).SelectOptimalPeer

	core.Println("PeerRegistry.SelectOptimalPeer")

	// Output: PeerRegistry.SelectOptimalPeer

}

func ExamplePeerRegistry_SelectNearestPeers() {

	_ = (*PeerRegistry).SelectNearestPeers

	core.Println("PeerRegistry.SelectNearestPeers")

	// Output: PeerRegistry.SelectNearestPeers

}

func ExamplePeerRegistry_FindNearby() {

	_ = (*PeerRegistry).FindNearby

	core.Println("PeerRegistry.FindNearby")

	// Output: PeerRegistry.FindNearby

}

func ExamplePeerRegistry_GetConnectedPeers() {

	_ = (*PeerRegistry).GetConnectedPeers

	core.Println("PeerRegistry.GetConnectedPeers")

	// Output: PeerRegistry.GetConnectedPeers

}

func ExamplePeerRegistry_ConnectedPeers() {

	_ = (*PeerRegistry).ConnectedPeers

	core.Println("PeerRegistry.ConnectedPeers")

	// Output: PeerRegistry.ConnectedPeers

}

func ExamplePeerRegistry_Count() {

	_ = (*PeerRegistry).Count

	core.Println("PeerRegistry.Count")

	// Output: PeerRegistry.Count

}

func ExamplePeerRegistry_Close() {

	_ = (*PeerRegistry).Close

	core.Println("PeerRegistry.Close")

	// Output: PeerRegistry.Close

}
