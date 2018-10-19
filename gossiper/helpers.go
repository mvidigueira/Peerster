package gossiper

import "github.com/mvidigueira/Peerster/dto"

//'have': a status packet representing what messages we have
//'want': a status packet representing what messages another peer is interested in
//returns: a status packet representing all the useful messages that can be provided to the peer
//Complexity: Assuming Go maps are ~O(1), complexity is ~O(N)
func peerStatusDifference(have []dto.PeerStatus, want []dto.PeerStatus) []dto.PeerStatus {
	wantMap := make(map[string]uint32)
	for _, ps := range want {
		wantMap[ps.Identifier] = ps.NextID
	}
	wantList := make([]dto.PeerStatus, 0)
	for _, ps := range have {
		want, ok := wantMap[ps.Identifier]
		if ok && (want < ps.NextID) {
			wantList = append(wantList, dto.PeerStatus{Identifier: ps.Identifier, NextID: want})
		} else if !ok {
			wantList = append(wantList, dto.PeerStatus{Identifier: ps.Identifier, NextID: 1})
		}
	}
	return wantList
}

//peerStatusUpdated - same as above, but includes messages the peer wants but we can't give them
//only used for printing according to project description example, otherwise pointless
func peerStatusUpdated(have []dto.PeerStatus, want []dto.PeerStatus) []dto.PeerStatus {
	wantMap := make(map[string]uint32)
	for _, ps := range want {
		wantMap[ps.Identifier] = ps.NextID
	}
	for _, ps := range have {
		_, ok := wantMap[ps.Identifier]
		if !ok {
			want = append(want, dto.PeerStatus{Identifier: ps.Identifier, NextID: 1})
		}
	}
	return want
}

//Complexity: ~O(N)
func stringArrayDifference(first []string, second []string) (difference []string) {
	mSecond := make(map[string]bool)
	for _, key := range second {
		mSecond[key] = true
	}
	difference = []string{}
	for _, key := range first {
		if _, ok := mSecond[key]; !ok {
			difference = append(difference, key)
		}
	}
	return
}
