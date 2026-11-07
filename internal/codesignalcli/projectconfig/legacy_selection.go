package projectconfig

import "errors"

func selectLegacy(dir, root, revision string) (Selection, error) {
	selection, err := canonicalOnDisk(root)
	if err != nil {
		return Selection{}, err
	}
	if _, err := Load(dir, revision, DefaultPath); err != nil {
		return legacyLoadFailure(selection, asDefaultPolicyError(err, DefaultPath))
	}
	selection.Path = DefaultPath
	selection.Deprecated = true
	return selection, nil
}

func legacyLoadFailure(selection Selection, err error) (Selection, error) {
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		return Selection{}, err
	}
	switch configErr.Kind {
	case KindNotFound, KindInvalid, KindUncommitted:
		return selection, nil
	default:
		return Selection{}, err
	}
}
