package providers

import (
	"github.com/uget/providers/basic"
	"github.com/uget/providers/nitroflare"
	"github.com/uget/providers/oboom"
	"github.com/uget/providers/rapidgator"
	"github.com/uget/providers/real_debrid"
	"github.com/uget/providers/share_online"
	"github.com/uget/providers/uploaded"
	"github.com/uget/providers/zippyshare"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/api"
)

var All = []api.Provider{
	&oboom.Provider{},
	&rapidgator.Provider{},
	&uploaded.Provider{},
	&zippyshare.Provider{},
	&share_online.Provider{},
	&nitroflare.Provider{},
	&real_debrid.Provider{},
	&basic.Provider{},
}

func init() {
	for _, p := range All {
		core.RegisterProvider(p)
	}
}
