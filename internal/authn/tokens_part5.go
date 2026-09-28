package authn

import (
	"fmt"

	"net/url"
)

func requireAbsoluteURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("authn: %s must be an absolute URL", field)
	}
	return nil
}
