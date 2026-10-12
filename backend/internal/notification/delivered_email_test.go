package notification

import (
	"net/url"
	"strings"
	"testing"
)

func deliveredFixture() OrderEmail {
	return OrderEmail{
		OrderNumber:   "HF-2026-81406187",
		PlacedAt:      "8 October 2026 · 15:06",
		Status:        "DELIVERED",
		PaymentLabel:  "Cash on delivery",
		DeliveryLabel: "Standard — 2 to 4 business days",
		CustomerName:  "Felix Forchu",
		CustomerEmail: "felix@example.com",
		CustomerPhone: "+237 6 54 90 47 07",
		AddressLines:  []string{"12 Rue Bonapriso", "Douala, CM"},
		Lines: []OrderLine{
			{Name: "Air Max 270", Variant: "EU 42 · Cream", Quantity: 2, Amount: "XAF 90,000"},
		},
		ItemCount:   2,
		Subtotal:    "XAF 90,000",
		DeliveryFee: "Complimentary",
		Total:       "XAF 90,000",
	}
}

// The delivered template is only ever exercised in production by a real
// delivery, so render it here: a template that fails to parse its own fields
// would otherwise surface as a silent log line after a courier's round.
func TestDeliveredEmailRenders(t *testing.T) {
	svc := newTestService(t)
	o := deliveredFixture()
	svc.applyStore(&o)

	html, err := svc.render(svc.deliveredOrder, templateData{
		Subject:  "Happy Feet — order HF-2026-81406187 delivered",
		Eyebrow:  "Order delivered",
		Headline: "Thank you, Felix.",
		Order:    o,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	for _, want := range []string{
		"HF-2026-81406187", // the order it is about
		"Air Max 270",      // what was delivered
		"XAF 90,000",       // the settled total
		"Happy Feet",       // store identity reached the template
		"Fourteen days",    // the returns promise
	} {
		if !strings.Contains(html, want) {
			t.Errorf("delivered HTML missing %q", want)
		}
	}

	text := deliveredPlainText(o)
	if !strings.Contains(text, "HF-2026-81406187") || !strings.Contains(text, "Air Max 270") {
		t.Errorf("delivered text alternative is missing the order detail:\n%s", text)
	}

	// The reply link must suit the moment. Reusing the order-placed wording
	// would invite a customer holding their shoes to ask us to confirm the
	// delivery that already happened.
	if strings.Contains(html, "just+placed+order") || strings.Contains(text, "just placed order") {
		t.Error("delivered email offers the order-placed WhatsApp message")
	}
	if !strings.Contains(text, "have+received+order") {
		t.Errorf("delivered email is missing the post-delivery WhatsApp message:\n%s", text)
	}
}

// Staff forward this link by hand when a customer has no email, so it has to
// carry the whole message and a dialable number.
func TestDeliveredWhatsAppLink(t *testing.T) {
	svc := newTestService(t)
	link := svc.ForwardLink(deliveredFixture())

	const wantNumber = "https://wa.me/237654904707?text="
	if !strings.HasPrefix(link, wantNumber) {
		t.Fatalf("link = %q, want prefix %q (spaces and + stripped)", link, wantNumber)
	}

	msg, err := url.QueryUnescape(strings.TrimPrefix(link, wantNumber))
	if err != nil {
		t.Fatalf("message is not valid percent-encoding: %v", err)
	}
	for _, want := range []string{"Felix", "HF-2026-81406187", "Happy Feet", "14 days"} {
		if !strings.Contains(msg, want) {
			t.Errorf("forwarded message missing %q:\n%s", want, msg)
		}
	}
}

// Without a phone there is nobody to open a chat with; the console relies on an
// empty string to hide the button rather than render a broken link.
func TestDeliveredWhatsAppLinkWithoutPhone(t *testing.T) {
	svc := newTestService(t)
	o := deliveredFixture()
	o.CustomerPhone = ""

	if link := svc.ForwardLink(o); link != "" {
		t.Errorf("expected no link without a customer phone, got %q", link)
	}
}

// The email and the WhatsApp forward must say the same thing — they are two
// deliveries of one message, and a customer may well receive both.
func TestDeliveredMessageIsShared(t *testing.T) {
	o := deliveredFixture()
	o.StoreName = "Happy Feet"

	msg := o.DeliveredMessage()
	if !strings.Contains(deliveredPlainText(o), msg) {
		t.Error("the text email does not carry the same words as the WhatsApp forward")
	}
}
