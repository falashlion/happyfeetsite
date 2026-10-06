package notification

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"

	"github.com/rs/zerolog"

	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/mailer"
)

//go:embed templates/*.html
var templateFS embed.FS

// EmailService renders and dispatches the store's transactional email.
//
// Each template is parsed once at construction against the shared layout, so a
// broken template surfaces at boot rather than on a customer's order.
type EmailService struct {
	mail  *mailer.Mailer
	store config.StoreConfig
	site  string
	log   zerolog.Logger

	merchantOrder *template.Template
	customerOrder *template.Template
	accountTmpl   *template.Template
}

// templateData is the root passed to every template.
type templateData struct {
	Subject   string
	Preheader string
	Eyebrow   string
	Headline  string
	Subhead   string
	Order     OrderEmail
	Account   AccountEmail
}

func NewEmailService(m *mailer.Mailer, cfg *config.Config, log zerolog.Logger) (*EmailService, error) {
	s := &EmailService{
		mail:  m,
		store: cfg.Store,
		site:  cfg.App.FrontendURL,
		log:   log.With().Str("component", "email").Logger(),
	}

	var err error
	if s.merchantOrder, err = parseWithLayout("templates/order_merchant.html"); err != nil {
		return nil, err
	}
	if s.customerOrder, err = parseWithLayout("templates/order_customer.html"); err != nil {
		return nil, err
	}
	if s.accountTmpl, err = parseWithLayout("templates/account.html"); err != nil {
		return nil, err
	}
	return s, nil
}

// parseWithLayout composes one body template with the shared layout. Because
// both files define "body", each needs its own template set.
func parseWithLayout(bodyPath string) (*template.Template, error) {
	t, err := template.New("email").ParseFS(templateFS, "templates/_layout.html", bodyPath)
	if err != nil {
		return nil, fmt.Errorf("notification: parse %s: %w", bodyPath, err)
	}
	return t, nil
}

// Enabled reports whether email can actually be delivered.
func (s *EmailService) Enabled() bool { return s.mail != nil && s.mail.Enabled() }

// applyStore threads store identity into the view model so templates and
// WhatsApp links never need config access.
func (s *EmailService) applyStore(o *OrderEmail) {
	o.StoreName = s.store.Name
	o.WhatsAppNumber = s.store.WhatsAppNumber
	o.WhatsAppDisplay = s.store.WhatsAppDisplay
	o.SupportEmail = s.store.SupportEmail
	o.StoreAddress = s.store.Address
	o.StoreHours = s.store.Hours
	o.SiteURL = s.site
}

// SendOrderPlaced fans out the two order emails: the merchant alert (always, to
// the configured owner address) and the customer receipt (when we hold an
// address for them). Delivery is asynchronous — order placement never waits on
// a mail relay.
func (s *EmailService) SendOrderPlaced(o OrderEmail) {
	if !s.Enabled() {
		s.log.Debug().Str("order", o.OrderNumber).Msg("email disabled; skipping order notifications")
		return
	}
	s.applyStore(&o)

	if s.store.OwnerEmail != "" {
		if msg, err := s.renderMerchant(o); err != nil {
			s.log.Error().Err(err).Str("order", o.OrderNumber).Msg("render merchant email")
		} else {
			s.mail.SendAsync(msg)
		}
	} else {
		s.log.Warn().Msg("STORE_OWNER_EMAIL is unset; merchant order alert not sent")
	}

	if o.CustomerEmail != "" {
		if msg, err := s.renderCustomer(o); err != nil {
			s.log.Error().Err(err).Str("order", o.OrderNumber).Msg("render customer email")
		} else {
			s.mail.SendAsync(msg)
		}
	}
}

func (s *EmailService) renderMerchant(o OrderEmail) (mailer.Message, error) {
	data := templateData{
		Subject: fmt.Sprintf("New order %s — %s to collect on delivery", o.OrderNumber, o.Total),
		Preheader: fmt.Sprintf("%s · %d item%s · %s payable on delivery",
			o.CustomerName, o.ItemCount, plural(o.ItemCount), o.Total),
		Eyebrow:  "New order placed",
		Headline: "A new order is in.",
		Subhead:  fmt.Sprintf("%s · %s", o.CustomerName, o.PlacedAt),
		Order:    o,
	}
	html, err := s.render(s.merchantOrder, data)
	if err != nil {
		return mailer.Message{}, err
	}
	return mailer.Message{
		To:      []string{s.store.OwnerEmail},
		ReplyTo: o.CustomerEmail,
		Subject: data.Subject,
		HTML:    html,
		Text:    merchantPlainText(o),
	}, nil
}

func (s *EmailService) renderCustomer(o OrderEmail) (mailer.Message, error) {
	data := templateData{
		Subject:   fmt.Sprintf("%s — your order %s is confirmed", s.store.Name, o.OrderNumber),
		Preheader: fmt.Sprintf("%s payable on delivery. We'll confirm your delivery window shortly.", o.Total),
		Eyebrow:   "Order confirmed",
		Headline:  "Thank you, " + firstName(o.CustomerName) + ".",
		Subhead:   "Your pairs are being prepared by hand.",
		Order:     o,
	}
	html, err := s.render(s.customerOrder, data)
	if err != nil {
		return mailer.Message{}, err
	}
	return mailer.Message{
		To:      []string{o.CustomerEmail},
		ReplyTo: s.store.SupportEmail,
		Subject: data.Subject,
		HTML:    html,
		Text:    customerPlainText(o),
	}, nil
}

func (s *EmailService) render(t *template.Template, data templateData) (string, error) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ── plain-text alternatives ───────────────────────────────────────────────────

func merchantPlainText(o OrderEmail) string {
	var b strings.Builder
	fmt.Fprintf(&b, "NEW ORDER %s\n%s\n\n", o.OrderNumber, o.PlacedAt)
	fmt.Fprintf(&b, "COLLECT ON DELIVERY: %s\n%s\n\n", o.Total, o.PaymentNote)
	fmt.Fprintf(&b, "Customer: %s\n", o.CustomerName)
	if o.CustomerPhone != "" {
		fmt.Fprintf(&b, "Phone: %s\n", o.CustomerPhone)
	}
	if o.CustomerEmail != "" {
		fmt.Fprintf(&b, "Email: %s\n", o.CustomerEmail)
	}
	b.WriteString("\nDeliver to:\n")
	for _, line := range o.AddressLines {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	fmt.Fprintf(&b, "\nDelivery: %s\n", o.DeliveryLabel)
	writeLines(&b, o)
	if link := o.MerchantWhatsAppLink(); link != "" {
		fmt.Fprintf(&b, "\nMessage the customer on WhatsApp:\n%s\n", link)
	}
	return b.String()
}

func customerPlainText(o OrderEmail) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Thank you, %s.\n\n", firstName(o.CustomerName))
	fmt.Fprintf(&b, "Order %s is confirmed.\nDue on delivery: %s\n\n", o.OrderNumber, o.Total)
	writeLines(&b, o)
	b.WriteString("\nDelivering to:\n")
	for _, line := range o.AddressLines {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	fmt.Fprintf(&b, "\n%s\n", o.DeliveryLabel)
	if link := o.CustomerWhatsAppLink(); link != "" {
		fmt.Fprintf(&b, "\nChat with us on WhatsApp (%s):\n%s\n", o.WhatsAppDisplay, link)
	}
	fmt.Fprintf(&b, "\n%s · %s · %s\n", o.StoreName, o.StoreAddress, o.SupportEmail)
	return b.String()
}

func writeLines(b *strings.Builder, o OrderEmail) {
	b.WriteString("\nItems:\n")
	for _, l := range o.Lines {
		fmt.Fprintf(b, "  %s — %s × %d — %s\n", l.Name, l.Variant, l.Quantity, l.Amount)
	}
	fmt.Fprintf(b, "\nSubtotal: %s\n", o.Subtotal)
	if o.Discount != "" {
		fmt.Fprintf(b, "Discount: -%s\n", o.Discount)
	}
	fmt.Fprintf(b, "Delivery: %s\nTotal:    %s\n", o.DeliveryFee, o.Total)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
