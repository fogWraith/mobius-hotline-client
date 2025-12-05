package style

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/gamut"
)

const (
	ColorLightGrey = lipgloss.Color("245")
	ColorCyan      = lipgloss.Color("63")
	ColorBrightRed = lipgloss.Color("196")
	ColorFuscia    = lipgloss.Color("170")
	ColorDarkGrey  = lipgloss.Color("241")
	ColorGrey2     = lipgloss.Color("235")
	ColorGrey3     = lipgloss.Color("236")
)

const Background1 = "☖"

// Styles
var (
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	// Banner styles
	HotkeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	// Styling for the main server screen title.
	ServerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorFuscia)

	SubTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 0, 1).
			Foreground(ColorFuscia)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorCyan).
			Height(2).
			Padding(0, 1)

	AdminUserStyle = lipgloss.NewStyle().
			Foreground(ColorBrightRed).
			Bold(true)

	AwayUserStyle = lipgloss.NewStyle().
			Faint(true)

	AwayAdminUserStyle = lipgloss.NewStyle().
				Foreground(ColorBrightRed).
				Bold(true).
				Faint(true)

	UsernameStyle = lipgloss.NewStyle().Bold(true)

	SubScreenStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorCyan). // Cyan border
			Background(ColorGrey2).      // Dark gray background
			Padding(1, 1)

	TaskWidgetStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorCyan).
			Padding(0, 1).
			Width(25)

	TaskActiveStyle = lipgloss.NewStyle().
			Foreground(ColorFuscia).
			Bold(true)

	TaskCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("2")) // Green

	TaskFailedStyle = lipgloss.NewStyle().
			Foreground(ColorBrightRed) // Red

	CategoryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFuscia)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFuscia)
)

var Subtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}

var DialogBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#874BFD")).
	Padding(1, 0).
	BorderTop(true).
	BorderLeft(true).
	BorderRight(true).
	BorderBottom(true)

var Blends = gamut.Blends(lipgloss.Color("#F25D94"), lipgloss.Color("#EDFF82"), 50)

func Rainbow(base lipgloss.Style, s string, colors []color.Color) string {
	var str string
	for i, ss := range s {
		c, _ := colorful.MakeColor(colors[i%len(colors)])
		str += base.Foreground(lipgloss.Color(c.Hex())).Render(string(ss))
	}
	return str
}

func RenderSubscreen(w, h int, title, content string) string {
	return lipgloss.Place(
		w,
		h,
		lipgloss.Center,
		lipgloss.Center,
		SubScreenStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				TitleStyle.Render(title),
				content,
			),
		),
		lipgloss.WithWhitespaceChars(Background1),
		lipgloss.WithWhitespaceForeground(Subtle),
	)

}
