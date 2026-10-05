package notification

import (
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/happyfeet/api/pkg/config"
	"github.com/happyfeet/api/pkg/mailer"
)

func TestFormatMoney(t *testing.T) {
	for _, tc := range []struct {
		amount   float64
		currency string
		want     string
	}{
		{0, "XAF", "XAF 0"},
		{999, "XAF", "XAF 999"},
		{1000, "XAF", "XAF 1,000"},
		{78000, "XAF", "XAF 78,000"},
		{234000, "XAF", "XAF 234,000"},
		{1234567, "XAF", "XAF 1,234,567"},
		{1500.6, "XAF", "XAF 1,501"}, // rounds to the nearest whole unit
		{2500, "", "XAF 2,500"},      // empty currency defaults to XAF
		{-1500, "XAF", "-XAF 1,500"},
	} {
		if got := FormatMoney(tc.amount, tc.currency); got != tc.want {
			t.Errorf("FormatMoney(%v, %q) = %q, want %q", tc.amount, tc.currency, got, tc.want)
		}
	}
}

func TestPaymentLabel(t *testing.T) {
	label, note := PaymentLabel("cash_on_delivery")
	if label != "Cash on delivery" {
		t.Errorf("label = %q", label)
	}
	if note == "" {
		t.Error("cash on delivery must carry a collection note")
	}
	if label, _ := PaymentLabel("mtn_momo"); label != "MTN Mobile Money" {
		t.Errorf("mtn label = %q", label)
	}
	// Unknown providers degrade to something readable rather than the raw enum.
	if label, _ := PaymentLabel("some_new_wallet"); label != "some new wallet" {
		t.Errorf("fallback label = %q", label)
	}
}

func TestDeliveryLabel(t *testing.T) {
	if got := DeliveryLabel("EXPRESS"); !strings.Contains(got, "Express") {
		t.Errorf("EXPRESS = %q", got)
	}
	if got := DeliveryLabel(""); !strings.Contains(got, "Standard") {
		t.Errorf("empty method should fall back to Standard, got %q", got)
	}
}

func TestCustomerWhatsAppLinkCarriesOrderContext(t *testing.T) {
	o := OrderEmail{
		OrderNumber:    "HF-2026-00012847",
		Total:          "XAF 234,000",
		PaymentLabel:   "Cash on delivery",
		StoreName:      "Happy Feet",
		WhatsAppNumber: "237612345678",
	}

	u, err := url.Parse(o.CustomerWhatsAppLink())
	if err != nil {
		t.Fatalf("link is not a valid URL: %v", err)
	}
	if u.Host != "wa.me" || u.Path != "/237612345678" {
		t.Errorf("unexpected target: %s%s", u.Host, u.Path)
	}
	msg := u.Query().Get("text")
	for _, want := range []string{"HF-2026-00012847", "XAF 234,000", "Happy Feet"} {
		if !strings.Contains(msg, want) {
			t.Errorf("prefilled message missing %q; got %q", want, msg)
		}
	}
}

func TestMerchantWhatsAppLinkUsesCustomerNumber(t *testing.T) {
	o := OrderEmail{
		OrderNumber:    "HF-1",
		Total:          "XAF 78,000",
		CustomerName:   "Amina Ngassa",
		CustomerPhone:  "+237 677 88 99 00",
		StoreName:      "Happy Feet",
		WhatsAppNumber: "237612345678",
	}
	u, err := url.Parse(o.MerchantWhatsAppLink())
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}
	// It must dial the *customer*, with the "+" and spaces stripped.
	if u.Path != "/237677889900" {
		t.Errorf("path = %q, want /237677889900", u.Path)
	}
	if msg := u.Query().Get("text"); !strings.Contains(msg, "Amina") {
		t.Errorf("message should greet the customer by first name; got %q", msg)
	}
}

func TestMerchantWhatsAppLinkEmptyWithoutPhone(t *testing.T) {
	o := OrderEmail{OrderNumber: "HF-1", WhatsAppNumber: "237612345678"}
	if got := o.MerchantWhatsAppLink(); got != "" {
		t.Errorf("no customer phone should yield no link, got %q", got)
	}
}

func TestWhatsAppLinkEmptyWhenStoreNumberUnset(t *testing.T) {
	o := OrderEmail{OrderNumber: "HF-1"}
	if got := o.CustomerWhatsAppLink(); got != "" {
		t.Errorf("expected empty link, got %q", got)
	}
}

// Rendering must survive the sparse case: no phone, no email, no discount.
// Those fields are all optional in the database.
func TestRenderOrderEmailsMinimalOrder(t *testing.T) {
	svc := newTestService(t)

	o := OrderEmail{
		OrderNumber:   "HF-2026-00000001",
		PlacedAt:      "27 August 2026 · 21:06",
		PaymentLabel:  "Cash on delivery",
		PaymentNote:   "Collect payment in full when the parcel is handed over.",
		DeliveryLabel: "Standard — 2 to 4 business days",
		CustomerName:  "Amina Ngassa",
		AddressLines:  []string{"14 Rue Njo-Njo", "Douala", "Cameroon"},
		Lines:         []OrderLine{{Name: "Lisbon Derby", Variant: "EU 42 · Cocoa", Quantity: 1, Amount: "XAF 78,000"}},
		ItemCount:     1,
		Subtotal:      "XAF 78,000",
		DeliveryFee:   "Complimentary",
		Total:         "XAF 78,000",
	}
	svc.applyStore(&o)

	merchant, err := svc.renderMerchant(o)
	if err != nil {
		t.Fatalf("renderMerchant: %v", err)
	}
	customer, err := svc.renderCustomer(o)
	if err != nil {
		t.Fatalf("renderCustomer: %v", err)
	}

	for _, m := range []mailer.Message{merchant, customer} {
		if m.HTML == "" || m.Text == "" {
			t.Error("both an HTML and a plain-text part are required")
		}
		if !strings.Contains(m.HTML, o.OrderNumber) {
			t.Errorf("HTML omits the order number:\n%s", truncate(m.HTML))
		}
		if !strings.Contains(m.HTML, "XAF 78,000") {
			t.Error("HTML omits the total")
		}
		if strings.Contains(m.HTML, "<no value>") {
			t.Error("template referenced a missing field")
		}
	}

	if merchant.To[0] != "owner@example.com" {
		t.Errorf("merchant alert went to %v", merchant.To)
	}
	// With no customer phone, the merchant email must not render a dead button.
	if strings.Contains(merchant.HTML, "wa.me/") {
		t.Error("merchant email should omit the WhatsApp button when there is no customer number")
	}
	// The customer copy always offers the store's own number.
	if !strings.Contains(customer.HTML, "wa.me/237612345678") {
		t.Error("customer email must carry the store WhatsApp link")
	}
}

func TestSendOrderPlacedIsNoopWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Store.OwnerEmail = "owner@example.com"
	svc, err := NewEmailService(mailer.NewSingle(cfg.SMTP, zerolog.Nop()), cfg, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled() {
		t.Fatal("no SMTP host configured, so the service must report disabled")
	}
	svc.SendOrderPlaced(OrderEmail{OrderNumber: "HF-1"}) // must not panic
}

func newTestService(t *testing.T) *EmailService {
	t.Helper()
	cfg := &config.Config{}
	cfg.App.FrontendURL = "http://localhost:3000"
	cfg.SMTP = config.SMTPConfig{Host: "localhost", Port: 1025, From: "orders@happyfeet.cm", FromName: "Happy Feet"}
	cfg.Store = config.StoreConfig{
		Name:            "Happy Feet",
		OwnerEmail:      "owner@example.com",
		WhatsAppNumber:  "237612345678",
		WhatsAppDisplay: "+237 6 12 34 56 78",
		SupportEmail:    "care@happyfeet.cm",
		Address:         "Bonapriso, Douala",
		Hours:           "Mon–Sat, 08:00–20:00 WAT",
	}
	svc, err := NewEmailService(mailer.NewSingle(cfg.SMTP, zerolog.Nop()), cfg, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewEmailService: %v", err)
	}
	return svc
}

func truncate(s string) string {
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
