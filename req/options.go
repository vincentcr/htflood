package req

import (
	"fmt"

	"github.com/imdario/mergo"
)

type Options struct {
	Debug    bool
	Insecure bool
	Pretty   bool
}

var options = Options{
	Debug:    false,
	Insecure: false,
}

func setOptions(o Options) error {
	if err := mergo.Merge(&options, o); err != nil {
		return fmt.Errorf("Failed to set options '%#v'", o, err)
	}

	if options.Insecure {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	return nil
}
