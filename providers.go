package providers

import (
	"github.com/uget/providers/basic"
	"github.com/uget/providers/oboom"
	"github.com/uget/providers/rapidgator"
	"github.com/uget/providers/real_debrid"
	"github.com/uget/providers/uploaded"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/api"
)

var All = []api.Provider{
	&basic.Provider{},
	&oboom.Provider{},
	&rapidgator.Provider{},
	&uploaded.Provider{},
	&real_debrid.Provider{},
}

func init() {
	for _, p := range All {
		core.RegisterProvider(p)
	}
}
