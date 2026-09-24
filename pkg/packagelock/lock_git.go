package packagelock

import (
	"net/url"
	"strings"

	"daml.com/x/assistant/pkg/gitparse"
	"daml.com/x/assistant/pkg/utils/stringset"
)

func addGitDarToDiffableMap(m map[string]stringset.StringSet, u *url.URL) {
	k := gitparse.GitLockKey(u)
	if _, ok := m[k]; !ok {
		m[k] = make(stringset.StringSet)
	}
	m[k].Add(gitparse.GitRefFromURI(u))
}

func gitRefIgnorableForSync(key, ref string) bool {
	return strings.HasPrefix(key, "git://") && gitparse.GitRefIsMutable(ref)
}
