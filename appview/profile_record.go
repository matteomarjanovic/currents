package main

import "time"

const currentsProfileNSID = "is.currents.actor.profile"

type repoBlobRef struct {
	Type     string            `json:"$type,omitempty"`
	Ref      map[string]string `json:"ref"`
	MimeType string            `json:"mimeType,omitempty"`
	Size     int               `json:"size,omitempty"`
}

type bskyActorProfile struct {
	DisplayName string       `json:"displayName"`
	Description string       `json:"description"`
	Avatar      *repoBlobRef `json:"avatar"`
	Banner      *repoBlobRef `json:"banner"`
}

type currentsProfileRecord struct {
	DisplayName string       `json:"displayName"`
	Description string       `json:"description"`
	Pronouns    string       `json:"pronouns"`
	Website     string       `json:"website"`
	Avatar      *repoBlobRef `json:"avatar"`
	Banner      *repoBlobRef `json:"banner"`
	CreatedAt   string       `json:"createdAt"`
}

func profileBlobURL(cdnBaseURL, did string, blob *repoBlobRef) string {
	if blob == nil {
		return ""
	}
	cid := blob.Ref["$link"]
	if cid == "" {
		return ""
	}
	return cdnBaseURL + "/img/" + did + "/" + cid
}

func currentsProfileFromBskyProfile(profile bskyActorProfile, createdAt string) currentsProfileRecord {
	return currentsProfileRecord{
		DisplayName: profile.DisplayName,
		Description: profile.Description,
		Avatar:      profile.Avatar,
		Banner:      profile.Banner,
		CreatedAt:   createdAt,
	}
}

func userRecordFromCurrentsProfile(did, handle, pdsEndpoint, cdnBaseURL string, profile currentsProfileRecord, fallbackCreatedAt time.Time) UserRecord {
	createdAt := fallbackCreatedAt
	if parsed := parseTimestamp(profile.CreatedAt); parsed != nil {
		createdAt = *parsed
	}
	return UserRecord{
		DID:         did,
		Handle:      handle,
		DisplayName: profile.DisplayName,
		Description: profile.Description,
		Pronouns:    profile.Pronouns,
		Website:     profile.Website,
		Avatar:      profileBlobURL(cdnBaseURL, did, profile.Avatar),
		Banner:      profileBlobURL(cdnBaseURL, did, profile.Banner),
		CreatedAt:   createdAt,
		PDSEndpoint: pdsEndpoint,
	}
}
