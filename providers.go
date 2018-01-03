package providers

import (
	// we only import. The init() function does the rest.
	_ "github.com/uget/providers/basic"
	_ "github.com/uget/providers/oboom"
	_ "github.com/uget/providers/rapidgator"
	_ "github.com/uget/providers/real_debrid"
	_ "github.com/uget/providers/uploaded"
)
