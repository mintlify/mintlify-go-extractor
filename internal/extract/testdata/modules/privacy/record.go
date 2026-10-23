package privacy

import (
	_ "embed"
	. "strings"
	"time"

	priv "corp.example/sdk/models"
	"example.com/privacy/sub"
	"github.com/public/go-thing"
	"github.com/public/widgets/v2"
)

// Record links to several packages. Its [corp.example/sdk/models.Model] stays private.
type Record struct {
	At      time.Time
	Sub     subpkg.Thing
	Private priv.Model
	Thing   thing.Value
	Widget  widgets.Widget
	Builder Builder
}
