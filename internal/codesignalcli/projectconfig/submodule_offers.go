package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// insideSubmoduleOffer is for a policy path inside a submodule checkout, or the submodule itself. The
// superproject records a submodule as one commit, so a file there can be neither restored nor committed
// here, a submodule is not a file, and a new policy written there would be refused as uncommittable: the
// way out is a policy this repository commits.
func insideSubmoduleOffer(submodule, repoPath string) restoreOffer {
	text := fmt.Sprintf(". It lies inside the submodule %q, which this repository records as a single commit, so a policy there can be neither restored nor committed here and Coach offers neither a restore nor a new policy; name a policy that is committed in this repository's own files", submodule)
	if submodule == repoPath {
		text = fmt.Sprintf(". It is the submodule %q, which this repository records as a single commit, so it cannot be a policy and Coach offers neither a restore nor a new policy; name a policy that is committed in this repository's own files", submodule)
	}
	return restoreOffer{remedy: projectreadiness.RemedyNone, cause: projectreadiness.CauseInSubmodule, text: text}
}

// submoduleCheckoutOffer is the offer for a directory on disk where the policy path is, when the index
// records a submodule exactly there: it is the submodule's checkout, and has the submodule's way out
// instead of "not a regular file".
func submoduleCheckoutOffer(judgment policyJudgment, found *ConfigError, repoPath string) (restoreOffer, bool) {
	if judgment.submodule == repoPath && found.PolicyCause == projectreadiness.CauseNotARegularFile {
		return insideSubmoduleOffer(judgment.submodule, repoPath), true
	}
	return restoreOffer{}, false
}
