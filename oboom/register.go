// +build !noOboom

package oboom

import "github.com/uget/uget/core"

func init() {
	core.RegisterProvider(&Provider{})
}
