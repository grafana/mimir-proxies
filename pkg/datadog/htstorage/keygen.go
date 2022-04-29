package htstorage

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const (
	hostCacheKeyTemplate = "ht:%d:%s:%s"

	cacheKeyTypeBase64  = 0
	cacheKeyTypeMD5Hash = 1

	maxCacheKeyLen = 250
)

var (
	hostCacheKeyBaseLen = len(fmt.Sprintf(hostCacheKeyTemplate, 0, "", ""))
)

//go:generate mockery --case underscore --inpackage --testonly --name CacheKeygen
type CacheKeygen interface {
	HostKey(orgID, hostName string) string
}

// MemcacheKeygen generates keys shorter than 250 characters and don't contain whitespaces
type MemcacheKeygen struct{}

func (MemcacheKeygen) HostKey(orgID, hostName string) string {
	orgIDBase64 := base64.StdEncoding.EncodeToString([]byte(orgID))
	hostNameBase64 := base64.StdEncoding.EncodeToString([]byte(hostName))

	if hostCacheKeyBaseLen+len(orgIDBase64)+len(hostNameBase64) <= maxCacheKeyLen {
		// happy path, orgID and hostName fit in the key len even being b64-ed
		return fmt.Sprintf(hostCacheKeyTemplate, cacheKeyTypeBase64, orgIDBase64, hostNameBase64)
	}

	hostSum := sha256.Sum256([]byte(hostName))
	hostNameMD5 := hex.EncodeToString(hostSum[:])

	orgSum := sha256.Sum256([]byte(orgID))
	orgIDMD5 := hex.EncodeToString(orgSum[:])

	return fmt.Sprintf(hostCacheKeyTemplate, cacheKeyTypeMD5Hash, orgIDMD5, hostNameMD5)
}
