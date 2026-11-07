package gitrepo

import "testing"

// remote.<name>.promisor is the whole key: a setting that merely starts with it, or has more after it, is another.
func TestASettingThatOnlyResemblesThePromisorKeyDoesNotMarkAPartialClone(t *testing.T) {
	for _, config := range []string{"remote.x.promisorfoo true\n", "xremote.x.promisor true\n", "remote.x.promisor.extra true\n"} {
		repository := fakeRepository{promisors: config}

		partial, err := IsPartialClone(repository.run, "repo")

		if err != nil || partial {
			t.Errorf("IsPartialClone(%q) = %v, %v; want an ordinary clone", config, partial, err)
		}
	}
}
