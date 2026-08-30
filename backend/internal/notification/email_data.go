package notification

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

// ── view models ───────────────────────────────────────────────────────────────
//
// These are deliberately flat, pre-formatted structs rather than domain types:
// email templates should never do arithmetic or locale work, and keeping the
// formatting here means the merchant copy and the customer copy always agree.

type OrderLine struct {
	Name     string
	Variant  string // "EU 42 · Onyx Black"
	Quantity int
	Amount   string // "XAF 145,000"
}

type OrderEmail struct {
	OrderNumber   string
	PlacedAt      string
	Status        string
	PaymentLabel  string // "Cash on delivery"
	PaymentNote   string
	DeliveryLabel string // "Standard — 2 to 4 days"

	CustomerName  string
	CustomerEmail string
	CustomerPhone string

	AddressLines []string

	Lines       []OrderLine
	Subtotal    string
	Discount    string // empty when none
	DeliveryFee string
	Total       string
	ItemCount   int

	// Store identity, threaded through so templates never read config.
	StoreName       string
	WhatsAppNumber  string
	WhatsAppDisplay string
	SupportEmail    string
	StoreAddress    string
	StoreHours      string
	SiteURL         string
}

// WhatsAppLink builds a wa.me deep link with a pre-filled message. The message
// differs by audience, so callers pass the body they want.
func (o OrderEmail) WhatsAppLink(message string) string {
	if o.WhatsAppNumber == "" {
		return ""
	}
	return "https://wa.me/" + o.WhatsAppNumber + "?text=" + url.QueryEscape(message)
}

// CustomerWhatsAppLink is the link the *customer* taps to reach the store.
func (o OrderEmail) CustomerWhatsAppLink() string {
	return o.WhatsAppLink(fmt.Sprintf(
		"Hello %s 👋\n\nI just placed order %s (%s, %s).\nI'd like to confirm my delivery.\n\nThank you!",
		o.StoreName, o.OrderNumber, o.Total, o.PaymentLabel))
}

// MerchantWhatsAppLink is the link the *store owner* taps to reach the customer.
// Without a customer phone number there is nobody to open a chat with, so it
// falls back to empty and the template omits the button.
func (o OrderEmail) MerchantWhatsAppLink() string {
	digits := digitsOnly(o.CustomerPhone)
	if digits == "" {
		return ""
	}
	msg := fmt.Sprintf(
		"Hello %s 👋\n\nThis is %s. Thank you for order %s (%s, payable on delivery).\nWe're confirming your delivery details now.",
		firstName(o.CustomerName), o.StoreName, o.OrderNumber, o.Total)
	return "https://wa.me/" + digits + "?text=" + url.QueryEscape(msg)
}

// OrderURL points the merchant at the order in the storefront account area.
func (o OrderEmail) OrderURL() string {
	if o.SiteURL == "" {
		return ""
	}
	return strings.TrimRight(o.SiteURL, "/") + "/account"
}

// ── formatting helpers ────────────────────────────────────────────────────────

// FormatMoney renders an amount the way the storefront does: currency code,
// thousands separated, no decimals (XAF has no minor unit).
func FormatMoney(amount float64, currency string) string {
	if currency == "" {
		currency = "XAF"
	}
	// math.Round, not `int64(amount + 0.5)`: the latter truncates toward zero
	// and turns -1500 into -1499 on refund/adjustment lines.
	whole := int64(math.Round(amount))
	sign := ""
	if whole < 0 {
		sign, whole = "-", -whole
	}
	digits := fmt.Sprintf("%d", whole)

	var out []byte
	for i, c := range []byte(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return fmt.Sprintf("%s%s %s", sign, currency, out)
}

func FormatDate(t time.Time) string {
	return t.Format("2 January 2006 · 15:04")
}

// PaymentLabel maps the storage enum onto merchant-readable copy.
func PaymentLabel(method string) (label, note string) {
	switch method {
	case "cash_on_delivery":
		return "Cash on delivery", "Collect payment in full when the parcel is handed over."
	case "mtn_momo":
		return "MTN Mobile Money", ""
	case "orange_money":
		return "Orange Money", ""
	case "stripe":
		return "Card", ""
	default:
		return strings.ReplaceAll(method, "_", " "), ""
	}
}

// DeliveryLabel maps the delivery enum onto customer-readable copy, mirroring
// the promises made on the checkout page.
func DeliveryLabel(method string) string {
	switch method {
	case "EXPRESS":
		return "Express — next business day"
	case "SAME_DAY":
		return "Same day — within Douala"
	default:
		return "Standard — 2 to 4 business days"
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstName(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return "there"
	}
	if i := strings.IndexByte(full, ' '); i > 0 {
		return full[:i]
	}
	return full
}
