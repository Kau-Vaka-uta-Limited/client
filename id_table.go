
package libkb

import (
	"fmt"
)

type TypedChainLink interface {
	GetRevocations() []*SigId
	insertIntoTable(tab *IdentityTable)
	GetSigId() SigId
	markRevoked(l TypedChainLink)
	ToString() string
}

type TypedChainLinkBase struct {
	*ChainLink
	revoked bool
}

type RemoteProofChainLink struct {
	TypedChainLinkBase
}

func (b TypedChainLinkBase) GetSigId() SigId {
	return b.unpacked.sigId
}

func (b TypedChainLinkBase) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(b)
}

func (b TypedChainLinkBase) markRevoked(r TypedChainLink) {
	b.revoked = true
}

func (b TypedChainLinkBase) ToString() string {
	return fmt.Sprintf("uid=%s, seq=%d, link=%s",
		string(b.parent.uid), b.unpacked.seqno, b.id.ToString())
}

func (l RemoteProofChainLink) TableKey() string {
	jw := l.payloadJson.AtKey("body").AtKey("service")
	if p, e2 := jw.AtKey("protocol").GetString(); e2 != nil {
		if p == "dns" || p == "http" || p == "https" {
			return "web"
		} else {
			return ""
		}
	} else if n, e3 := jw.AtKey("name").GetString(); e3 != nil {
		return n
	} else {
		G.Log.Warning("Unknown signature @%s", l.ToString())
		return ""
	}
}

func (l RemoteProofChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(l)
	if k := l.TableKey(); len(k) > 0 {
		v, found := tab.remoteProofs[k]
		if !found {
			v = make([]RemoteProofChainLink, 0, 1)
		}
		v = append(v, l)
		tab.remoteProofs[k] = v
	}
}

func (l TrackChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(l)
	whom, err := l.payloadJson.AtPath("body.track.basics.username").GetString()
	if err != nil {
		G.Log.Warning("Bad track statement @%s: %s", l.ToString(), err.Error())
	} else {
		tab.tracks[whom] = l
	}
}

func (u UntrackChainLink) insertIntoTable(tab *IdentityTable) {
	tab.insertLink(u)
	whom, err := u.payloadJson.AtPath("body.untrack.basics.username").GetString()
	if err != nil {
		G.Log.Warning("Bad untrack statement @%s: %s", u.ToString(), err.Error())
	} else if tobj, found := tab.tracks[whom]; !found {
		G.Log.Notice("Bad untrack of %s; no previous tracking statement found",
			whom)
	} else {
		tobj.untrack = &u
	}
}

type TrackChainLink struct {
	TypedChainLinkBase
	untrack *UntrackChainLink
}

type CryptocurrencyChainLink struct {
	TypedChainLinkBase
}

type RevokeChainLink struct {
	TypedChainLinkBase
}

type UntrackChainLink struct {
	TypedChainLinkBase
}

type SelfSigChainLink struct {
	TypedChainLinkBase
}

type IdentityTable struct {
	sigChain        *SigChain
	revocations     map[SigId]bool
	links           map[SigId]TypedChainLink
	remoteProofs    map[string][]RemoteProofChainLink
	tracks          map[string]TrackChainLink
	order           []TypedChainLink
}

func (tab *IdentityTable) insertLink(l TypedChainLink) {
	tab.links[l.GetSigId()] = l
	tab.order = append(tab.order, l)
	for _, rev := range(l.GetRevocations()) {
		tab.revocations[*rev] = true
		if targ, found := tab.links[*rev]; !found {
			G.Log.Warning("Can't revoke signature %s @%s",
				rev.ToString(true), l.ToString())
		} else {
			targ.markRevoked(l)
		}
	}
}

func NewTypedChainLink(cl *ChainLink) (ret TypedChainLink, err error) {
	var s string
	s, err = cl.payloadJson.AtKey("body").AtKey("type").GetString()
	if err != nil {
		return
	}

	base := TypedChainLinkBase { cl, false }
	switch s {
	case "web_service_binding":
		ret = RemoteProofChainLink { base }
	case "track" :
		ret = TrackChainLink { base, nil }
	case "untrack" :
		ret = UntrackChainLink { base }
	case "cryptocurrency":
		ret = CryptocurrencyChainLink { base }
	case "revoke":
		ret = RevokeChainLink { base }
	case "self_sig":
		ret = SelfSigChainLink { base }
	default:
		err = fmt.Errorf("Unknown signature type %s @%s", s, base.ToString())
	}
	return
}

func NewIdentityTable(sc *SigChain) *IdentityTable {
	ret := &IdentityTable{
		sigChain : sc,
		revocations : make(map[SigId]bool),
		links : make(map[SigId]TypedChainLink),
		remoteProofs : make(map[string][]RemoteProofChainLink),
		tracks : make(map[string]TrackChainLink),
		order : make([]TypedChainLink, 0, sc.Len()),
	}
	ret.Populate()
	return ret
}

func (idt *IdentityTable) Populate() {
	G.Log.Debug("+ Populate ID Table")
	for _,link := range(idt.sigChain.chainLinks) {
		tl, err := NewTypedChainLink(link)
		if err != nil {
			G.Log.Warning(err.Error())
		} else {
			tl.insertIntoTable(idt)
		}
	}
	G.Log.Debug("- Populate ID Table")
}

