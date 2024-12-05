package tui

import (
	"context"
	"math"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	terminal "github.com/terminaldotshop/terminal-sdk-go"
	"github.com/terminaldotshop/terminal/go/pkg/api"
	"github.com/terminaldotshop/terminal/go/pkg/tui/theme"
)

type (
	page = int
	size = int
)

const (
	menuPage page = iota
	splashPage
	shopPage
	accountPage
	paymentPage
	cartPage
	subscribePage
	shippingPage
	confirmPage
	finalPage
	subscriptionsPage
	ordersPage
	aboutPage
	faqPage
)

const (
	undersized size = iota
	small
	medium
	large
)

type model struct {
	ready           bool
	switched        bool
	page            page
	hasMenu         bool
	checkout        bool
	state           state
	context         context.Context
	client          *terminal.Client
	user            terminal.User
	accountPages    []page
	products        []terminal.Product
	addresses       []terminal.Shipping
	cards           []terminal.Card
	subscriptions   []terminal.Subscription
	orders          []terminal.Order
	cart            terminal.Cart
	subscription    terminal.SubscriptionNewParams
	renderer        *lipgloss.Renderer
	theme           theme.Theme
	fingerprint     string
	promotion       string
	viewportWidth   int
	viewportHeight  int
	widthContainer  int
	heightContainer int
	widthContent    int
	heightContent   int
	size            size
	accessToken     string
	faqs            []FAQ
	viewport        viewport.Model
	hasScroll       bool
	error           *VisibleError
}

type state struct {
	splash        SplashState
	cursor        cursorState
	shipping      shippingState
	subscriptions subscriptionsState
	orders        ordersState
	shop          shopState
	account       accountState
	footer        footerState
	cart          cartState
	subscribe     subscribeState
	payment       paymentState
	confirm       confirmState
	faq           faqState
	menu          menuState
}

type children struct{}

func NewModel(
	renderer *lipgloss.Renderer,
	fingerprint string,
	promotion string,
) (tea.Model, error) {
	api.Init()

	ctx := context.Background()

	result := model{
		context:     ctx,
		page:        splashPage,
		renderer:    renderer,
		fingerprint: fingerprint,
		promotion:   promotion,
		theme:       theme.BasicTheme(renderer, nil),
		faqs:        LoadFaqs(),
		accountPages: []page{
			ordersPage,
			subscriptionsPage,
			// shippingPage,
			// paymentPage,
			faqPage,
			aboutPage,
		},
		subscription: terminal.SubscriptionNewParams{},
		state: state{
			splash: SplashState{},
			shop: shopState{
				selected: 0,
			},
			cart: cartState{
				selected: 0,
			},
			subscribe: subscribeState{
				selected: 0,
			},
			account: accountState{
				selected: 0,
			},
			subscriptions: subscriptionsState{
				selected: 0,
			},
			orders: ordersState{
				selected: 0,
			},
			payment: paymentState{
				input: paymentInput{},
			},
			shipping: shippingState{
				input: shippingInput{
					country: "US",
				},
			},
			footer: footerState{
				commands: []footerCommand{},
			},
		},
	}

	result, _ = result.FaqInit()
	return result, nil
}

func (m model) Init() tea.Cmd {
	return m.SplashInit()
}

func (m model) SwitchPage(page page) model {
	m.page = page
	m.switched = true
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		m.error = &VisibleError{
			message: api.GetErrorMessage(msg),
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.viewportWidth = msg.Width
		m.viewportHeight = msg.Height

		switch {
		case m.viewportWidth < 20 || m.viewportHeight < 10:
			m.size = undersized
			m.widthContainer = m.viewportWidth
			m.heightContainer = m.viewportHeight
		case m.viewportWidth < 40:
			m.size = small
			m.widthContainer = m.viewportWidth
			m.heightContainer = m.viewportHeight
		case m.viewportWidth < 60:
			m.size = medium
			m.widthContainer = 40
			m.heightContainer = int(math.Min(float64(msg.Height), 30))
		default:
			m.size = large
			m.widthContainer = 60
			m.heightContainer = int(math.Min(float64(msg.Height), 30))
		}

		m.widthContent = m.widthContainer - 4
		m = m.updateViewport()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.error != nil {
				m.error = nil
				return m, nil
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	case CursorTickMsg:
		m, cmd := m.CursorUpdate(msg)
		// TODO: this is bad, but otherwise the cursor doesn't blink
		m.viewport.SetContent(m.getContent())
		return m, cmd
	case CartUpdatedMsg:
		if m.state.cart.lastUpdateID == msg.updateID {
			m.cart = msg.updated
		}
	case terminal.UserInitResponseResult:
		m.user = msg.User
		m.products = msg.Products
		m.cart = msg.Cart
		m.cards = msg.Cards
		m.addresses = msg.Addresses
		m.subscriptions = msg.Subscriptions
		m.orders = msg.Orders
		m = m.reorderProducts()
	case terminal.User:
		m.user = msg
	case []terminal.Product:
		m.products = msg
		m = m.reorderProducts()
	case terminal.Cart:
		m.cart = msg
	case []terminal.Card:
		m.cards = msg
	case []terminal.Shipping:
		m.addresses = msg
	case []terminal.Subscription:
		m.subscriptions = msg
	case []terminal.Order:
		m.orders = msg
	}

	var cmd tea.Cmd
	switch m.page {
	case menuPage:
		m, cmd = m.MenuUpdate(msg)
	case splashPage:
		m, cmd = m.SplashUpdate(msg)
	case accountPage:
		m, cmd = m.AccountUpdate(msg)
	case aboutPage:
		m, cmd = m.AboutUpdate(msg)
	case shopPage:
		m, cmd = m.ShopUpdate(msg)
	case cartPage:
		m, cmd = m.CartUpdate(msg)
	case subscribePage:
		m, cmd = m.SubscribeUpdate(msg)
	case paymentPage:
		m, cmd = m.PaymentUpdate(msg)
	case shippingPage:
		m, cmd = m.ShippingUpdate(msg)
	case confirmPage:
		m, cmd = m.ConfirmUpdate(msg)
	case finalPage:
		m, cmd = m.FinalUpdate(msg)
	}

	var headerCmd tea.Cmd
	m, headerCmd = m.HeaderUpdate(msg)
	cmds := []tea.Cmd{headerCmd}

	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.hasMenu = m.page == shopPage ||
		m.page == accountPage
		// m.page == aboutPage ||
		// m.page == faqPage

	m.checkout = m.page == cartPage ||
		m.page == subscribePage ||
		m.page == paymentPage ||
		m.page == shippingPage ||
		m.page == confirmPage

	m.viewport.SetContent(m.getContent())
	m.viewport, cmd = m.viewport.Update(msg)
	if m.switched {
		m = m.updateViewport()
		m.switched = false
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.size == undersized {
		return m.ResizeView()
	}

	if m.error != nil {
		return m.ErrorView()
	}

	switch m.page {
	case splashPage:
		return m.SplashView()
	case menuPage:
		return m.MenuView()
	default:
		header := m.HeaderView()
		footer := m.FooterView()
		breadcrumbs := m.BreadcrumbsView()
		content := m.viewport.View()

		var view string
		if m.hasScroll {
			view = lipgloss.JoinHorizontal(
				lipgloss.Top,
				content,
				m.theme.Base().Width(1).Render(), // space between content and scrollbar
				m.getScrollbar(),
			)
		} else {
			view = m.getContent()
		}

		height := m.heightContainer
		height -= lipgloss.Height(header)
		height -= lipgloss.Height(breadcrumbs)
		height -= lipgloss.Height(footer)

		child := lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			breadcrumbs,
			m.theme.Base().
				Width(m.widthContainer).
				Height(height).
				Padding(0, 1).
				Render(view),
			footer,
		)

		return m.renderer.Place(
			m.viewportWidth,
			m.viewportHeight,
			lipgloss.Center,
			lipgloss.Center,
			m.theme.Base().
				MaxWidth(m.widthContainer).
				MaxHeight(m.heightContainer).
				Render(child),
		)
	}
}

func (m model) getContent() string {
	page := "unknown"
	switch m.page {
	case shopPage:
		page = m.ShopView()
	case cartPage:
		page = m.CartView()
	case subscribePage:
		page = m.SubscribeView()
	case paymentPage:
		page = m.PaymentView()
	case shippingPage:
		page = m.ShippingView(m.widthContent-2, false)
	case confirmPage:
		page = m.ConfirmView()
	case finalPage:
		page = m.FinalView()
	case accountPage:
		page = m.AccountView()
	}
	return page
}

func (m model) getScrollbar() string {
	y := m.viewport.YOffset
	vh := m.viewport.Height
	ch := lipgloss.Height(m.getContent())
	if vh >= ch {
		return ""
	}

	height := (vh * vh) / ch
	maxScroll := ch - vh
	nYP := 1.0 - (float64(y) / float64(maxScroll))
	if nYP <= 0 {
		nYP = 1
	} else if nYP >= 1 {
		nYP = 0
	}

	bar := m.theme.Base().
		Height(height).
		Width(1).
		Background(m.theme.Accent()).
		Render()

	style := m.theme.Base().Width(1).Height(vh)

	return style.Render(lipgloss.PlaceVertical(vh, lipgloss.Position(nYP), bar))
}

var modifiedKeyMap = viewport.KeyMap{
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "½ page up"),
	),
	HalfPageDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "½ page down"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
}

func (m model) updateViewport() model {
	headerHeight := lipgloss.Height(m.HeaderView())
	breadcrumbsHeight := lipgloss.Height(m.BreadcrumbsView())
	footerHeight := lipgloss.Height(m.FooterView())
	verticalMarginHeight := headerHeight + footerHeight + breadcrumbsHeight + 2

	width := m.widthContainer - 4
	m.heightContent = m.heightContainer - verticalMarginHeight

	if !m.ready {
		m.viewport = viewport.New(width, m.heightContent)
		m.viewport.YPosition = headerHeight
		m.viewport.HighPerformanceRendering = false
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = m.heightContent
		m.viewport.GotoTop()
	}

	if m.page == faqPage ||
		m.page == aboutPage ||
		m.page == finalPage {
		m.viewport.KeyMap = viewport.DefaultKeyMap()
	} else {
		m.viewport.KeyMap = modifiedKeyMap
	}

	m.hasScroll = m.viewport.VisibleLineCount() < m.viewport.TotalLineCount()

	if m.hasScroll {
		m.widthContent = m.widthContainer - 4
	} else {
		m.widthContent = m.widthContainer - 2
	}

	return m
}
