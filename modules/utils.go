package modules

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	MAX_NAME_LIMIT         = 63
	RESOURCE_NAME_HASH_LEN = 8
)

func CloneAndMergeMaps(m1, m2 map[string]string) map[string]string {
	res := map[string]string{}
	for k, v := range m1 {
		res[k] = v
	}
	for k, v := range m2 {
		res[k] = v
	}
	return res
}

func MustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func SafeName(name string, suffix string, maxLen int) string {
	const randomHashLen = 6
	// remove suffix if already there.
	name = strings.TrimSuffix(name, suffix)

	if len(name) <= maxLen-len(suffix) {
		return name + suffix
	}

	val := sha256.Sum256([]byte(name))
	hash := fmt.Sprintf("%x", val)
	suffix = fmt.Sprintf("-%s%s", hash[:randomHashLen], suffix)

	// truncate and make room for the suffix. also trim any leading, trailing
	// hyphens to prevent '--' (not allowed in deployment names).
	truncLen := maxLen - len(suffix)
	truncated := name[0:truncLen]
	truncated = strings.Trim(truncated, "-")
	return truncated + suffix
}

func slug(input string) string {
	return strings.ToLower(
		regexp.MustCompile(`[^\w -]+`).ReplaceAllString(
			regexp.MustCompile(` +`).ReplaceAllString(
				regexp.MustCompile(`_+`).ReplaceAllString(input, "-"),
				"-"),
			""))
}

func BuildResourceName(kind, name, projectID string, limit int) string {
	if limit == 0 {
		limit = MAX_NAME_LIMIT
	}

	nameComponents := []string{projectID, name, kind}
	sluggedName := slug(name)
	initialName := slug(strings.Join(nameComponents, "-"))
	charSizeToRemove := limit - len(initialName)
	hashLength := RESOURCE_NAME_HASH_LEN

	hash := md5.Sum([]byte(sluggedName))
	hashStr := hex.EncodeToString(hash[:])[:hashLength]

	var qualifiedName string
	if charSizeToRemove >= 0 {
		qualifiedName = sluggedName
	} else {
		qualifiedName = fmt.Sprintf("%s-%s", sluggedName[:len(sluggedName)+(charSizeToRemove+-(hashLength+1))], hashStr)
	}

	return slug(strings.Join([]string{projectID, qualifiedName, kind}, "-"))
}
