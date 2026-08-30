// HappyFeet — translation dictionary. EN is authoritative; FR is hand-authored
// to keep the editorial voice (atelier / concierge / luxury without affectation).
//
// Keys are flat strings (e.g. "checkout.step1.title") so neither client nor
// server has to walk a deep tree. Use t(key) — falls back to the EN string,
// then to the key, so a missing key is loud rather than invisible.

export const SUPPORTED_LOCALES = ["en", "fr"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";

export const LOCALE_LABEL: Record<Locale, string> = { en: "EN", fr: "FR" };
export const LOCALE_NAME: Record<Locale, string> = { en: "English", fr: "Français" };

type Dict = Record<string, string>;

const en: Dict = {
  // ── Common ─────────────────────────────────────────────────────────────────
  "common.brand": "Happy Feet",
  "common.tagline": "Step into luxury.",
  "common.complimentary": "Complimentary",
  "common.loading": "One moment…",
  "common.continue": "Continue",
  "common.cancel": "Cancel",
  "common.back": "Back",
  "common.signin": "Sign in",
  "common.register": "Become a member",
  "common.signout": "Sign out",
  "common.pair_one": "pair",
  "common.pair_other": "pairs",
  "common.optional": "optional",

  // ── Promo strip ────────────────────────────────────────────────────────────
  "promo.line": "Complimentary courier over XAF 50,000 · Forty-eight hours across Cameroon · Fourteen-day returns",
  "promo.shipping_free": "Complimentary courier over XAF 50,000",
  "promo.delivery_time": "Forty-eight hours across Cameroon",
  "promo.returns": "Fourteen-day returns",

  // ── Header / nav ───────────────────────────────────────────────────────────
  "nav.women": "Women",
  "nav.men": "Men",
  "nav.kids": "Kids",
  "nav.sneakers": "Sneakers",
  "nav.boots": "Boots",
  "nav.sale": "The Atelier Sale",
  "nav.search": "Search",
  "nav.account": "Account",
  "nav.bag": "Bag",
  "nav.menu": "Menu",
  "nav.close": "Close",
  "nav.section_shop": "Shop",
  "nav.section_account": "Your account",
  "nav.your_bag": "Your bag",
  "nav.your_bag_short": "Your bag",
  "nav.sign_in_register": "Sign in / register",
  "nav.language": "Language",
  "nav.lang_en": "English",
  "nav.lang_fr": "Français",

  // ── Footer ─────────────────────────────────────────────────────────────────
  "footer.about_blurb":
    "An online luxury footwear maison — fifteen houses, every silhouette, couriered from Douala within forty-eight hours.",
  "footer.col_shop": "Shop",
  "footer.col_house": "The house",
  "footer.col_care": "Care",
  "footer.col_newsletter": "Newsletter",
  "footer.newsletter_blurb": "Five new arrivals, every Wednesday. Nothing else.",
  "footer.newsletter_placeholder": "Your email",
  "footer.newsletter_cta": "Subscribe",
  "footer.house.story": "Our story",
  "footer.house.materials": "Materials",
  "footer.house.concierge": "Concierge sizing",
  "footer.house.stores": "Stores",
  "footer.care.courier": "Complimentary courier",
  "footer.care.returns": "Fourteen-day returns",
  "footer.care.sizing": "Sizing guide",
  "footer.care.contact": "Contact",
  "footer.copyright": "© Happy Feet, Douala · All rights reserved",
  "footer.legal.privacy": "Privacy",
  "footer.legal.terms": "Terms",
  "footer.legal.cookies": "Cookies",

  // ── Homepage ───────────────────────────────────────────────────────────────
  "home.eyebrow": "The 2026 collection",
  "home.h1_l1": "Step into",
  "home.h1_l2": "luxury.",
  "home.lede":
    "Hand-curated footwear from fifteen houses, finished to last a decade, couriered in forty-eight hours.",
  "home.cta_browse": "Browse the collection",
  "home.cta_hero_product": "See the boot of the season",
  "home.promise.truck.title": "Complimentary courier",
  "home.promise.truck.body": "Forty-eight hours across Cameroon. Tracked from our hands to yours.",
  "home.promise.returns.title": "Fourteen-day returns",
  "home.promise.returns.body": "In the original box, with the dust-bag, and we refund in full.",
  "home.promise.authentic.title": "Authentic, every pair",
  "home.promise.authentic.body": "Sourced directly from the house or its authorised distributor.",
  "home.promise.sizing.title": "Concierge sizing",
  "home.promise.sizing.body": "A real human will help you choose. Try WhatsApp or the chat.",
  "home.section.shopby_eyebrow": "Shop by category",
  "home.section.shopby_title": "Where will they take you?",
  "home.section.shopby_action": "View all categories",
  "home.section.new_eyebrow": "New this week",
  "home.section.new_title": "Forty-six new arrivals. Curated, not collected.",
  "home.section.new_action": "View all",
  "home.editorial.eyebrow": "The atelier",
  "home.editorial.quote":
    "“A shoe should answer to the foot, never the other way around.”",
  "home.editorial.body":
    "Each pair we list is hand-vetted by our atelier in Douala — leather grain, stitch density, sole bond — before it ever sees the courier. If we wouldn’t wear it, we won’t sell it.",
  "home.editorial.cta": "Read our standards",
  "home.section.favs_eyebrow": "House favourites",
  "home.section.favs_title": "What our members keep coming back for.",
  "home.cat.sneakers.label": "Sneakers",
  "home.cat.sneakers.sub": "On foot, every day",
  "home.cat.loafers.label": "Loafers",
  "home.cat.loafers.sub": "Slip on, slip out",
  "home.cat.boots.label": "Boots",
  "home.cat.boots.sub": "Built to last",
  "home.cat.formal.label": "Formal & dress",
  "home.cat.formal.sub": "For the occasion",

  // ── PLP ────────────────────────────────────────────────────────────────────
  "plp.crumb_root": "Happy Feet",
  "plp.eyebrow": "The {{category}} collection",
  "plp.title.all": "All shoes.",
  "plp.title.sale": "The Atelier Sale.",
  "plp.title.sneakers": "Sneakers, with the patience of leather.",
  "plp.title.boots": "Boots that outlast a season.",
  "plp.title.loafers": "Loafers, shaped to a single foot.",
  "plp.title.formal": "Formal & dress — for the occasion.",
  "plp.title.sandals": "Sandals, paper-cool.",
  "plp.brand": "Brand",
  "plp.brand_all": "All",
  "plp.sort": "Sort",
  "plp.sort.relevance": "Relevance",
  "plp.sort.price_asc": "Price · low to high",
  "plp.sort.price_desc": "Price · high to low",
  "plp.sort.rating": "Highest rated",
  "plp.count": "Showing {{n}} {{noun}}",
  "plp.count.in": "in",
  "plp.empty":
    "No pairs to show. Try clearing the filters, or browse another category.",
  "plp.sale_name": "The Atelier Sale",
  "plp.all_name": "All shoes",

  // ── PDP ────────────────────────────────────────────────────────────────────
  "pdp.brand": "Brand",
  "pdp.materials_eyebrow": "Materials",
  "pdp.materials_title": "Built from",
  "pdp.reviews_eyebrow": "Member reviews",
  "pdp.reviews_count": "{{count}} reviews · {{rating}} stars",
  "pdp.review_quote":
    "“Sized perfectly, arrived in two days, and the box alone is worth keeping.”",
  "pdp.review_meta": "Falash M. · Verified purchase · 12 days ago",
  "pdp.picker.color": "Colour",
  "pdp.picker.size_eu": "Size · EU",
  "pdp.picker.sizing_guide": "Sizing guide",
  "pdp.picker.size_format": "EU {{eu}} · US {{us}} · UK {{uk}}",
  "pdp.cta.select_size": "Select a size",
  "pdp.cta.add": "Add to bag",
  "pdp.cta.added": "Added to bag",
  "pdp.save_aria": "Save for later",
  "pdp.gallery_image_aria": "View image {{n}}",
  "pdp.promise.courier": "Complimentary courier — forty-eight hours across Cameroon.",
  "pdp.promise.returns": "Fourteen days to return, in the original box, refunded in full.",
  "pdp.promise.authentic":
    "Sourced direct from the house or its authorised distributor.",

  // ── Bag ────────────────────────────────────────────────────────────────────
  "bag.metadata_title": "Your bag",
  "bag.eyebrow": "Your bag",
  "bag.reserved_one": "{{n}} pair reserved.",
  "bag.reserved_other": "{{n}} pairs reserved.",
  "bag.size": "Size",
  "bag.qty_decrease": "Decrease",
  "bag.qty_increase": "Increase",
  "bag.remove": "Remove",
  "bag.summary": "Order summary",
  "bag.subtotal": "Subtotal",
  "bag.courier": "Courier",
  "bag.total": "Total",
  "bag.continue": "Continue to payment",
  "bag.add_more": "Add {{amount}} more for complimentary courier.",
  "bag.payment_line": "MTN Mobile Money · Orange Money · Card · Cash on delivery",
  "bag.empty_title": "Your bag is empty.",
  "bag.empty_lede": "Forty-six new arrivals this week — where would you like to begin?",
  "bag.empty_cta": "Browse the collection",

  // ── Coupons ────────────────────────────────────────────────────────────────
  "coupon.placeholder": "Have a code?",
  "coupon.apply": "Apply",
  "coupon.remove": "Remove",
  "coupon.try_ours": "Try ours",
  "coupon.error.empty": "Enter a code first.",
  "coupon.error.unknown": "{{code}} isn’t a code we recognise.",
  "coupon.error.min": "Add {{amount}} more to unlock this code.",
  "coupon.error.members": "This code is reserved for Atelier members.",
  "coupon.WELCOME10.label": "10% off your first order",
  "coupon.WELCOME10.description": "First-order bonus · stacks with the seasonal sale.",
  "coupon.ATELIER20.label": "20% off above XAF 30,000",
  "coupon.ATELIER20.description":
    "Spend 30,000 or more and the atelier knocks 20% off.",
  "coupon.FREESHIP.label": "Complimentary courier",
  "coupon.FREESHIP.description": "Replaces the courier fee with our compliments.",
  "coupon.XAF5000.label": "XAF 5,000 off above 25,000",
  "coupon.XAF5000.description": "A flat 5,000 off carts over 25,000.",
  "coupon.GOLD25.label": "Member-only · 25% off",
  "coupon.GOLD25.description": "Reserved for Atelier members.",
  "coupon.REFRIEND.label": "XAF 10,000 off — friend referral",
  "coupon.REFRIEND.description": "Use a friend's referral code.",

  // ── Checkout ───────────────────────────────────────────────────────────────
  "checkout.metadata_title": "Checkout",
  "checkout.back_bag": "Back to your bag",
  "checkout.title": "Checkout",
  "checkout.step1_eyebrow": "Step one",
  "checkout.step1_title": "Delivery address",
  "checkout.step2_eyebrow": "Step two",
  "checkout.step2_title": "Delivery method",
  "checkout.step3_eyebrow": "Step three",
  "checkout.step3_title": "Payment",
  "checkout.use_saved": "Use a saved address instead",
  "checkout.addr.recipient": "Recipient",
  "checkout.addr.label": "Label",
  "checkout.addr.street": "Street",
  "checkout.addr.city": "City",
  "checkout.addr.country": "Country (ISO-2)",
  "checkout.addr.phone": "Phone (optional, E.164)",
  "checkout.addr.add_another": "Add another address",
  "checkout.addr.default": "Default",
  "checkout.delivery.standard": "Standard courier",
  "checkout.delivery.standard_sub": "Forty-eight hours across Cameroon",
  "checkout.delivery.express": "Express",
  "checkout.delivery.express_sub": "Next-morning Douala / Yaoundé",
  "checkout.delivery.sameday": "Same-day Douala",
  "checkout.delivery.sameday_sub": "Order before noon, hands-on by sunset",
  "checkout.payment.mtn": "MTN Mobile Money",
  "checkout.payment.mtn_sub": "A USSD prompt arrives on your phone",
  "checkout.payment.orange": "Orange Money",
  "checkout.payment.orange_sub": "We send you to Orange’s checkout",
  "checkout.payment.card": "Card · Visa / Mastercard",
  "checkout.payment.card_sub": "Secure card capture via Stripe",
  "checkout.payment.cod": "Cash on delivery",
  "checkout.payment.cod_sub": "Pay the courier at the door",
  "checkout.payment.momo_number": "Mobile money number",
  "checkout.payment.cod_only_title": "Pay when your order reaches you.",
  "checkout.payment.cod_due": "Due on delivery",
  "checkout.payment.cod_point_1": "Nothing is charged now — you settle with the courier at your door.",
  "checkout.payment.cod_point_2": "Cash in XAF, exact amount appreciated.",
  "checkout.payment.cod_point_3": "Inspect the box before you pay. Wrong pair, no payment.",
  "checkout.payment.cod_disabled_note":
    "Mobile money and card payment are being finalised and will return shortly.",
  "checkout.summary.title": "Your order",
  "checkout.cta.place_order": "Place order",
  "checkout.cta.placing": "Placing order…",
  "checkout.terms":
    "You agree to the order terms and our 14-day returns policy.",
  "checkout.error.address": "Please complete the delivery address.",
  "checkout.error.sku": "Your bag has a legacy item without a SKU. Remove it and re-add from the product page.",
  "checkout.empty_title": "Your bag is empty.",
  "checkout.empty_lede": "Add a pair before you check out.",
  "checkout.empty_cta": "Browse the collection",
  "checkout.signin_title": "Sign in to complete your order.",
  "checkout.signin_lede": "Your bag and any code you applied will be waiting.",

  // ── Confirmation ───────────────────────────────────────────────────────────
  "confirmation.metadata_title": "Order confirmed",
  "confirmation.eyebrow": "Order confirmed",
  "confirmation.title": "Thank you. Your shoes are on their way.",
  "confirmation.subline_no_order": "Order —",
  "confirmation.body":
    "We’ve sent the receipt to your contact on file. The atelier is preparing your order — our courier picks up within forty-eight hours and you’ll get a tracking link the moment it leaves Douala.",
  "confirmation.cod_eyebrow": "Payable on delivery",
  "confirmation.cod_body":
    "Nothing has been charged. Have this amount ready in cash when our courier arrives — you pay only once the box is in your hands.",
  "confirmation.cod_point_1": "We call to confirm your delivery window.",
  "confirmation.cod_point_2": "Inspect the pair at the door, then settle with the courier.",
  "confirmation.momo_eyebrow": "Complete your MTN MoMo payment",
  "confirmation.momo_dial": "Dial this on your phone:",
  "confirmation.momo_ref": "Ref · {{ref}}",
  "confirmation.checkout_eyebrow": "Complete your payment",
  "confirmation.checkout_body":
    "You’ll be redirected to finish paying on a secure page.",
  "confirmation.checkout_cta": "Go to checkout",
  "confirmation.keep_browsing": "Keep browsing",
  "confirmation.track": "Track this order",
  "confirmation.receipt_title": "Receipt",
  "confirmation.missing_ref": "Missing order reference.",
  "confirmation.load_error": "Could not load the order.",

  // ── Auth / account ─────────────────────────────────────────────────────────
  "auth.metadata_title": "Become a member",
  "auth.eyebrow.welcome": "Welcome back",
  "auth.eyebrow.member": "Become a member",
  "auth.side.welcome": "Step back\ninto the atelier.",
  "auth.side.member": "An atelier\nfor the few.",
  "auth.side.welcome_lede":
    "Your bag, your wishlist, your reservations — exactly where you left them.",
  "auth.side.member_lede":
    "Order history, reservations, and the concierge — all in one place.",
  "auth.feature.courier": "Complimentary courier across Cameroon",
  "auth.feature.returns": "Fourteen days to return, in the box",
  "auth.feature.authentic": "Sourced direct from the house",
  "auth.feature.concierge": "Concierge sizing on WhatsApp",
  "auth.tab.signin": "Sign in",
  "auth.tab.register": "Become a member",
  "auth.form_title.signin": "Welcome back.",
  "auth.form_title.register": "Step into Happy Feet.",
  "auth.form_sub.signin":
    "Pick up where you left off — bag, wishlist, reservations.",
  "auth.form_sub.register": "A few details and you're in. We keep it brief.",
  "auth.channel.email": "Email",
  "auth.channel.phone": "Phone",
  "auth.field.first_name": "First name",
  "auth.field.last_name": "Last name",
  "auth.field.email": "Email",
  "auth.field.phone": "Phone (E.164)",
  "auth.field.password": "Password",
  "auth.field.password_placeholder.signin": "••••••••",
  "auth.field.password_placeholder.register": "At least 8 characters",
  "auth.field.password_helper": "Minimum 8 characters, 1 uppercase, 1 digit.",
  "auth.placeholder.first_name": "Daniel",
  "auth.placeholder.last_name": "Mensah",
  "auth.placeholder.email": "you@example.com",
  "auth.placeholder.phone": "+237612345678",
  "auth.or": "or",
  "auth.google.signing_in": "Signing you in…",
  "auth.google.error": "Google sign-in could not be completed. Try again, or use your email.",
  "auth.google.welcome_new": "Welcome to Happy Feet, {{name}}.",
  "auth.keep_signed": "Keep me signed in",
  "auth.forgot": "Forgot your password?",
  "auth.consent":
    "Send me the occasional editorial and new-arrivals note. I can unsubscribe any time.",
  "auth.cta.signin": "Sign in",
  "auth.cta.register": "Become a member",
  "auth.footnote.to_register": "New here?",
  "auth.footnote.create_link": "Create an account",
  "auth.footnote.to_signin": "Already a member?",
  "auth.footnote.signin_link": "Sign in",
  "auth.fineprint.lead": "By continuing you agree to our",
  "auth.fineprint.terms": "terms",
  "auth.fineprint.and": "and",
  "auth.fineprint.privacy": "privacy policy",
  "auth.error.both_names": "First and last name are required.",
  "auth.error.generic": "Something went wrong. Try again in a moment.",

  "account.signedin.welcome_back": "Welcome back, {{name}}.",
  "account.signedin.lede":
    "Order history, reservations, and the concierge — coming next.",
  "account.signedin.meta": "Signed in as {{contact}} · {{role}}",
  "account.signedin.signing_out": "Signing out…",

  // ── Not found ──────────────────────────────────────────────────────────────
  "notfound.eyebrow": "404",
  "notfound.title": "This page has been put back on the shelf.",
  "notfound.body": "It either moved, or it was never there.",
  "notfound.cta": "Return to Happy Feet",

  // ── About ──────────────────────────────────────────────────────────────────
  "about.metadata_title": "Our standards",
  "about.eyebrow": "The atelier",
  "about.title":
    "A shoe should answer to the foot, never the other way around.",
  "about.lede":
    "Each pair we list is hand-vetted by our atelier in Douala — leather grain, stitch density, sole bond — before it ever sees the courier.",
  "about.p1":
    "Happy Feet is an online luxury footwear maison serving Cameroon and West & Central Africa, with fifteen houses across every silhouette — sneakers to dress derbies, loafers to cork-soled sandals.",
  "about.p2":
    "We courier within forty-eight hours, accept returns for fourteen days in the original box, and only stock pairs sourced directly from the house or its authorised distributor.",
  "about.cta": "Browse the collection",

  // ── WhatsApp ───────────────────────────────────────────────────────────────
  // The cta.* keys compose the pre-filled message; {{brand}}, {{order}} and
  // {{amount}} are substituted before the wa.me link is built.
  "wa.cta.eyebrow": "Direct line",
  "wa.cta.title": "Anything else? Message us.",
  "wa.cta.body":
    "A change of address, a different size, a question about your delivery — one tap and you are talking to the atelier, not a queue.",
  "wa.cta.button": "Chat with us on WhatsApp",
  "wa.cta.hours": "Every day, 08:00–22:00 WAT",
  "wa.cta.greeting": "Hello {{brand}} 👋",
  "wa.cta.with_order":
    "I've just placed order {{order}} ({{amount}}) and would like to confirm my delivery.",
  "wa.cta.without_order": "I'd like some help with my order, please.",
  "wa.fab_aria": "Chat with us on WhatsApp",
  "wa.dialog_aria": "Concierge chat",
  "wa.eyebrow": "Concierge",
  "wa.title": "How may we help?",
  "wa.hours": "Reply within minutes, every day from 08:00 to 22:00 WAT.",
  "wa.topic.order_tracking": "Track an order",
  "wa.topic.sizing": "Sizing help",
  "wa.topic.returns": "Start a return",
  "wa.topic.concierge": "Book the concierge",
  "wa.topic.general": "Something else",
  "wa.separator": "— or write to us —",
  "wa.placeholder": "A note for the concierge…",
  "wa.send": "Send via WhatsApp",
  "wa.msg.header": "Bonjour Happy Feet,",
  "wa.msg.footer_sent_from": "Sent from happyfeet.com",
  "wa.msg.sizing": "I need a hand choosing the right size. Could the concierge help?",
  "wa.msg.concierge":
    "I would like to book the concierge — three pairs picked for me. Happy to send three photos of the shoes I wear most.",
  "wa.msg.order_tracking": "I would like an update on a recent order.",
  "wa.msg.returns": "I would like to start a return.",
  "wa.msg.support_intro": "I need a hand with the following:",
  "wa.msg.general": "I have a question for the concierge.",
};

const fr: Dict = {
  // ── Common ─────────────────────────────────────────────────────────────────
  "common.brand": "Happy Feet",
  "common.tagline": "Entrez dans le luxe.",
  "common.complimentary": "Offerte",
  "common.loading": "Un instant…",
  "common.continue": "Continuer",
  "common.cancel": "Annuler",
  "common.back": "Retour",
  "common.signin": "Se connecter",
  "common.register": "Devenir membre",
  "common.signout": "Se déconnecter",
  "common.pair_one": "paire",
  "common.pair_other": "paires",
  "common.optional": "facultatif",

  // ── Promo strip ────────────────────────────────────────────────────────────
  "promo.line":
    "Livraison offerte au-delà de 50 000 XAF · Quarante-huit heures au Cameroun · Retours sous quatorze jours",
  "promo.shipping_free": "Livraison offerte au-delà de 50 000 XAF",
  "promo.delivery_time": "Quarante-huit heures au Cameroun",
  "promo.returns": "Retours sous quatorze jours",

  // ── Header / nav ───────────────────────────────────────────────────────────
  "nav.women": "Femme",
  "nav.men": "Homme",
  "nav.kids": "Enfant",
  "nav.sneakers": "Baskets",
  "nav.boots": "Bottes",
  "nav.sale": "La Vente de l’Atelier",
  "nav.search": "Rechercher",
  "nav.account": "Compte",
  "nav.bag": "Panier",
  "nav.menu": "Menu",
  "nav.close": "Fermer",
  "nav.section_shop": "Boutique",
  "nav.section_account": "Votre compte",
  "nav.your_bag": "Votre panier",
  "nav.your_bag_short": "Votre panier",
  "nav.sign_in_register": "Connexion / inscription",
  "nav.language": "Langue",
  "nav.lang_en": "English",
  "nav.lang_fr": "Français",

  // ── Footer ─────────────────────────────────────────────────────────────────
  "footer.about_blurb":
    "Maison en ligne de souliers de luxe — quinze maisons, toutes les silhouettes, livrées depuis Douala en quarante-huit heures.",
  "footer.col_shop": "Boutique",
  "footer.col_house": "La maison",
  "footer.col_care": "Service",
  "footer.col_newsletter": "Lettre",
  "footer.newsletter_blurb":
    "Cinq nouveautés, chaque mercredi. Rien de plus.",
  "footer.newsletter_placeholder": "Votre adresse e-mail",
  "footer.newsletter_cta": "S’inscrire",
  "footer.house.story": "Notre histoire",
  "footer.house.materials": "Matières",
  "footer.house.concierge": "Conseil pointure",
  "footer.house.stores": "Boutiques",
  "footer.care.courier": "Livraison offerte",
  "footer.care.returns": "Retours sous 14 jours",
  "footer.care.sizing": "Guide des tailles",
  "footer.care.contact": "Nous contacter",
  "footer.copyright": "© Happy Feet, Douala · Tous droits réservés",
  "footer.legal.privacy": "Confidentialité",
  "footer.legal.terms": "Conditions",
  "footer.legal.cookies": "Cookies",

  // ── Homepage ───────────────────────────────────────────────────────────────
  "home.eyebrow": "La collection 2026",
  "home.h1_l1": "Entrez dans",
  "home.h1_l2": "le luxe.",
  "home.lede":
    "Souliers sélectionnés à la main de quinze maisons, finis pour durer une décennie, livrés en quarante-huit heures.",
  "home.cta_browse": "Parcourir la collection",
  "home.cta_hero_product": "Voir la botte de la saison",
  "home.promise.truck.title": "Livraison offerte",
  "home.promise.truck.body":
    "Quarante-huit heures au Cameroun. Suivie de nos mains aux vôtres.",
  "home.promise.returns.title": "Retours sous quatorze jours",
  "home.promise.returns.body":
    "Dans la boîte d’origine, avec le sac à poussière — remboursés intégralement.",
  "home.promise.authentic.title": "Authentique, chaque paire",
  "home.promise.authentic.body":
    "Sourcée directement auprès de la maison ou de son distributeur agréé.",
  "home.promise.sizing.title": "Conseil pointure",
  "home.promise.sizing.body":
    "Un conseiller vous aide à choisir — sur WhatsApp ou dans la messagerie.",
  "home.section.shopby_eyebrow": "Par catégorie",
  "home.section.shopby_title": "Où vous mèneront-elles ?",
  "home.section.shopby_action": "Voir toutes les catégories",
  "home.section.new_eyebrow": "Nouveautés cette semaine",
  "home.section.new_title":
    "Quarante-six nouveautés. Choisies, non collectionnées.",
  "home.section.new_action": "Tout voir",
  "home.editorial.eyebrow": "L’atelier",
  "home.editorial.quote":
    "« Un soulier doit répondre au pied, jamais l’inverse. »",
  "home.editorial.body":
    "Chaque paire est validée à la main par notre atelier de Douala — grain du cuir, densité de la couture, collage de la semelle — avant de partir chez le coursier. Si nous ne la portions pas, nous ne la vendrions pas.",
  "home.editorial.cta": "Lire nos exigences",
  "home.section.favs_eyebrow": "Coups de cœur de la maison",
  "home.section.favs_title": "Ce que nos membres reprennent sans cesse.",
  "home.cat.sneakers.label": "Baskets",
  "home.cat.sneakers.sub": "Aux pieds, chaque jour",
  "home.cat.loafers.label": "Mocassins",
  "home.cat.loafers.sub": "Enfilez, repartez",
  "home.cat.boots.label": "Bottes",
  "home.cat.boots.sub": "Faites pour durer",
  "home.cat.formal.label": "Ville & habillé",
  "home.cat.formal.sub": "Pour les grandes occasions",

  // ── PLP ────────────────────────────────────────────────────────────────────
  "plp.crumb_root": "Happy Feet",
  "plp.eyebrow": "La collection {{category}}",
  "plp.title.all": "Tous les souliers.",
  "plp.title.sale": "La Vente de l’Atelier.",
  "plp.title.sneakers": "Baskets, avec la patience du cuir.",
  "plp.title.boots": "Bottes faites pour durer une saison de plus.",
  "plp.title.loafers": "Mocassins, taillés pour un seul pied.",
  "plp.title.formal": "Ville & habillé — pour les grandes occasions.",
  "plp.title.sandals": "Sandales, fraîches comme le papier.",
  "plp.brand": "Marque",
  "plp.brand_all": "Toutes",
  "plp.sort": "Tri",
  "plp.sort.relevance": "Pertinence",
  "plp.sort.price_asc": "Prix · croissant",
  "plp.sort.price_desc": "Prix · décroissant",
  "plp.sort.rating": "Mieux notés",
  "plp.count": "{{n}} {{noun}} affichée·s",
  "plp.count.in": "en",
  "plp.empty":
    "Aucune paire à afficher. Effacez les filtres ou explorez une autre catégorie.",
  "plp.sale_name": "La Vente de l’Atelier",
  "plp.all_name": "Tous les souliers",

  // ── PDP ────────────────────────────────────────────────────────────────────
  "pdp.brand": "Marque",
  "pdp.materials_eyebrow": "Matières",
  "pdp.materials_title": "Composition",
  "pdp.reviews_eyebrow": "Avis des membres",
  "pdp.reviews_count": "{{count}} avis · {{rating}} étoiles",
  "pdp.review_quote":
    "« Pointure parfaite, livrée en deux jours, et la boîte mérite d’être conservée. »",
  "pdp.review_meta": "Falash M. · Achat vérifié · il y a 12 jours",
  "pdp.picker.color": "Couleur",
  "pdp.picker.size_eu": "Pointure · EU",
  "pdp.picker.sizing_guide": "Guide des tailles",
  "pdp.picker.size_format": "EU {{eu}} · US {{us}} · UK {{uk}}",
  "pdp.cta.select_size": "Choisir une pointure",
  "pdp.cta.add": "Ajouter au panier",
  "pdp.cta.added": "Ajouté au panier",
  "pdp.save_aria": "Enregistrer pour plus tard",
  "pdp.gallery_image_aria": "Voir l’image {{n}}",
  "pdp.promise.courier": "Livraison offerte — quarante-huit heures au Cameroun.",
  "pdp.promise.returns":
    "Quatorze jours pour retourner, dans la boîte d’origine — remboursés intégralement.",
  "pdp.promise.authentic":
    "Sourcés directement auprès de la maison ou de son distributeur agréé.",

  // ── Bag ────────────────────────────────────────────────────────────────────
  "bag.metadata_title": "Votre panier",
  "bag.eyebrow": "Votre panier",
  "bag.reserved_one": "{{n}} paire réservée.",
  "bag.reserved_other": "{{n}} paires réservées.",
  "bag.size": "Pointure",
  "bag.qty_decrease": "Diminuer",
  "bag.qty_increase": "Augmenter",
  "bag.remove": "Retirer",
  "bag.summary": "Récapitulatif",
  "bag.subtotal": "Sous-total",
  "bag.courier": "Livraison",
  "bag.total": "Total",
  "bag.continue": "Passer au paiement",
  "bag.add_more":
    "Ajoutez {{amount}} pour bénéficier de la livraison offerte.",
  "bag.payment_line":
    "MTN Mobile Money · Orange Money · Carte · Paiement à la livraison",
  "bag.empty_title": "Votre panier est vide.",
  "bag.empty_lede":
    "Quarante-six nouveautés cette semaine — par où voulez-vous commencer ?",
  "bag.empty_cta": "Parcourir la collection",

  // ── Coupons ────────────────────────────────────────────────────────────────
  "coupon.placeholder": "Un code ?",
  "coupon.apply": "Appliquer",
  "coupon.remove": "Retirer",
  "coupon.try_ours": "Essayez les nôtres",
  "coupon.error.empty": "Saisissez d’abord un code.",
  "coupon.error.unknown": "{{code}} n’est pas un code reconnu.",
  "coupon.error.min": "Ajoutez {{amount}} pour activer ce code.",
  "coupon.error.members": "Code réservé aux membres Atelier.",
  "coupon.WELCOME10.label": "−10 % sur votre première commande",
  "coupon.WELCOME10.description":
    "Bonus de bienvenue · cumulable avec la vente de saison.",
  "coupon.ATELIER20.label": "−20 % au-delà de 30 000 XAF",
  "coupon.ATELIER20.description":
    "Dépensez 30 000 ou plus, l’atelier retire 20 %.",
  "coupon.FREESHIP.label": "Livraison offerte",
  "coupon.FREESHIP.description":
    "Remplace les frais de livraison par nos compliments.",
  "coupon.XAF5000.label": "−5 000 XAF au-delà de 25 000",
  "coupon.XAF5000.description":
    "Cinq mille XAF de remise dès 25 000 d’achats.",
  "coupon.GOLD25.label": "Membres seulement · −25 %",
  "coupon.GOLD25.description": "Réservé aux membres Atelier.",
  "coupon.REFRIEND.label": "−10 000 XAF — parrainage",
  "coupon.REFRIEND.description": "Code de parrainage d’un ami.",

  // ── Checkout ───────────────────────────────────────────────────────────────
  "checkout.metadata_title": "Paiement",
  "checkout.back_bag": "Retour au panier",
  "checkout.title": "Paiement",
  "checkout.step1_eyebrow": "Étape un",
  "checkout.step1_title": "Adresse de livraison",
  "checkout.step2_eyebrow": "Étape deux",
  "checkout.step2_title": "Mode de livraison",
  "checkout.step3_eyebrow": "Étape trois",
  "checkout.step3_title": "Paiement",
  "checkout.use_saved": "Choisir une adresse enregistrée",
  "checkout.addr.recipient": "Destinataire",
  "checkout.addr.label": "Libellé",
  "checkout.addr.street": "Rue",
  "checkout.addr.city": "Ville",
  "checkout.addr.country": "Pays (ISO-2)",
  "checkout.addr.phone": "Téléphone (facultatif, E.164)",
  "checkout.addr.add_another": "Ajouter une autre adresse",
  "checkout.addr.default": "Par défaut",
  "checkout.delivery.standard": "Coursier standard",
  "checkout.delivery.standard_sub": "Quarante-huit heures au Cameroun",
  "checkout.delivery.express": "Express",
  "checkout.delivery.express_sub": "Le lendemain matin · Douala / Yaoundé",
  "checkout.delivery.sameday": "Le jour même · Douala",
  "checkout.delivery.sameday_sub":
    "Commandez avant midi, livré avant le coucher du soleil",
  "checkout.payment.mtn": "MTN Mobile Money",
  "checkout.payment.mtn_sub": "Une demande USSD arrive sur votre téléphone",
  "checkout.payment.orange": "Orange Money",
  "checkout.payment.orange_sub": "Nous vous redirigeons vers Orange",
  "checkout.payment.card": "Carte · Visa / Mastercard",
  "checkout.payment.card_sub": "Saisie sécurisée via Stripe",
  "checkout.payment.cod": "Paiement à la livraison",
  "checkout.payment.cod_sub": "Réglez le coursier à votre porte",
  "checkout.payment.momo_number": "Numéro mobile money",
  "checkout.payment.cod_only_title": "Réglez à la réception de votre commande.",
  "checkout.payment.cod_due": "À régler à la livraison",
  "checkout.payment.cod_point_1":
    "Rien n’est débité maintenant — vous réglez le coursier à votre porte.",
  "checkout.payment.cod_point_2": "En espèces, en XAF ; l’appoint est apprécié.",
  "checkout.payment.cod_point_3":
    "Inspectez la boîte avant de payer. Mauvaise paire, aucun paiement.",
  "checkout.payment.cod_disabled_note":
    "Le mobile money et la carte sont en cours de finalisation et reviendront prochainement.",
  "checkout.summary.title": "Votre commande",
  "checkout.cta.place_order": "Valider la commande",
  "checkout.cta.placing": "Validation en cours…",
  "checkout.terms":
    "En continuant, vous acceptez nos conditions et notre politique de retours sous 14 jours.",
  "checkout.error.address": "Veuillez compléter l’adresse de livraison.",
  "checkout.error.sku":
    "Votre panier contient un article sans SKU. Retirez-le et ré-ajoutez depuis la fiche produit.",
  "checkout.empty_title": "Votre panier est vide.",
  "checkout.empty_lede": "Ajoutez une paire avant de payer.",
  "checkout.empty_cta": "Parcourir la collection",
  "checkout.signin_title": "Connectez-vous pour finaliser la commande.",
  "checkout.signin_lede":
    "Votre panier et le code appliqué vous attendent.",

  // ── Confirmation ───────────────────────────────────────────────────────────
  "confirmation.metadata_title": "Commande confirmée",
  "confirmation.eyebrow": "Commande confirmée",
  "confirmation.title": "Merci. Vos souliers sont en route.",
  "confirmation.subline_no_order": "Commande —",
  "confirmation.body":
    "Nous avons envoyé le reçu à votre contact. L’atelier prépare votre commande — notre coursier passe sous quarante-huit heures et vous recevrez un lien de suivi dès le départ de Douala.",
  "confirmation.cod_eyebrow": "Payable à la livraison",
  "confirmation.cod_body":
    "Rien n’a été débité. Préparez ce montant en espèces à l’arrivée de notre coursier — vous ne réglez qu’une fois la boîte entre vos mains.",
  "confirmation.cod_point_1": "Nous appelons pour confirmer votre créneau de livraison.",
  "confirmation.cod_point_2": "Inspectez la paire à la porte, puis réglez le coursier.",
  "confirmation.momo_eyebrow": "Finaliser votre paiement MTN MoMo",
  "confirmation.momo_dial": "Composez ceci sur votre téléphone :",
  "confirmation.momo_ref": "Réf · {{ref}}",
  "confirmation.checkout_eyebrow": "Finaliser le paiement",
  "confirmation.checkout_body":
    "Vous serez redirigé pour terminer le paiement sur une page sécurisée.",
  "confirmation.checkout_cta": "Aller au paiement",
  "confirmation.keep_browsing": "Poursuivre la visite",
  "confirmation.track": "Suivre cette commande",
  "confirmation.receipt_title": "Reçu",
  "confirmation.missing_ref": "Référence de commande manquante.",
  "confirmation.load_error": "Impossible de charger la commande.",

  // ── Auth / account ─────────────────────────────────────────────────────────
  "auth.metadata_title": "Devenir membre",
  "auth.eyebrow.welcome": "Heureux de vous revoir",
  "auth.eyebrow.member": "Devenir membre",
  "auth.side.welcome": "Retrouvez\nl’atelier.",
  "auth.side.member": "Un atelier\npour quelques-uns.",
  "auth.side.welcome_lede":
    "Votre panier, votre liste de souhaits, vos réservations — là où vous les aviez laissés.",
  "auth.side.member_lede":
    "Historique, réservations, conciergerie — réunis en un seul endroit.",
  "auth.feature.courier": "Livraison offerte dans tout le Cameroun",
  "auth.feature.returns": "Quatorze jours pour retourner, dans la boîte",
  "auth.feature.authentic": "Sourcés directement auprès de la maison",
  "auth.feature.concierge": "Conseil pointure sur WhatsApp",
  "auth.tab.signin": "Se connecter",
  "auth.tab.register": "Devenir membre",
  "auth.form_title.signin": "Heureux de vous revoir.",
  "auth.form_title.register": "Entrez chez Happy Feet.",
  "auth.form_sub.signin":
    "Reprenez là où vous étiez — panier, souhaits, réservations.",
  "auth.form_sub.register":
    "Quelques informations et c’est fait. Nous restons brefs.",
  "auth.channel.email": "E-mail",
  "auth.channel.phone": "Téléphone",
  "auth.field.first_name": "Prénom",
  "auth.field.last_name": "Nom",
  "auth.field.email": "E-mail",
  "auth.field.phone": "Téléphone (E.164)",
  "auth.field.password": "Mot de passe",
  "auth.field.password_placeholder.signin": "••••••••",
  "auth.field.password_placeholder.register": "8 caractères minimum",
  "auth.field.password_helper":
    "8 caractères minimum, dont une majuscule et un chiffre.",
  "auth.placeholder.first_name": "Daniel",
  "auth.placeholder.last_name": "Mensah",
  "auth.placeholder.email": "vous@example.com",
  "auth.placeholder.phone": "+237612345678",
  "auth.or": "ou",
  "auth.google.signing_in": "Connexion en cours…",
  "auth.google.error": "La connexion Google n’a pas abouti. Réessayez, ou utilisez votre e-mail.",
  "auth.google.welcome_new": "Bienvenue chez Happy Feet, {{name}}.",
  "auth.keep_signed": "Rester connecté",
  "auth.forgot": "Mot de passe oublié ?",
  "auth.consent":
    "Envoyez-moi occasionnellement les nouveautés et lectures de l’atelier. Je peux me désabonner à tout moment.",
  "auth.cta.signin": "Se connecter",
  "auth.cta.register": "Devenir membre",
  "auth.footnote.to_register": "Nouveau ?",
  "auth.footnote.create_link": "Créer un compte",
  "auth.footnote.to_signin": "Déjà membre ?",
  "auth.footnote.signin_link": "Se connecter",
  "auth.fineprint.lead": "En continuant, vous acceptez nos",
  "auth.fineprint.terms": "conditions",
  "auth.fineprint.and": "et notre",
  "auth.fineprint.privacy": "politique de confidentialité",
  "auth.error.both_names": "Le prénom et le nom sont requis.",
  "auth.error.generic": "Une erreur est survenue. Réessayez dans un instant.",

  "account.signedin.welcome_back": "Heureux de vous revoir, {{name}}.",
  "account.signedin.lede":
    "Historique des commandes, réservations, conciergerie — bientôt disponibles.",
  "account.signedin.meta": "Connecté en tant que {{contact}} · {{role}}",
  "account.signedin.signing_out": "Déconnexion…",

  // ── Not found ──────────────────────────────────────────────────────────────
  "notfound.eyebrow": "404",
  "notfound.title": "Cette page a regagné l’étagère.",
  "notfound.body": "Elle a été déplacée, ou n’a jamais existé.",
  "notfound.cta": "Retour à Happy Feet",

  // ── About ──────────────────────────────────────────────────────────────────
  "about.metadata_title": "Nos exigences",
  "about.eyebrow": "L’atelier",
  "about.title":
    "Un soulier doit répondre au pied, jamais l’inverse.",
  "about.lede":
    "Chaque paire est validée à la main par notre atelier de Douala — grain du cuir, densité de la couture, collage de la semelle — avant de partir chez le coursier.",
  "about.p1":
    "Happy Feet est une maison en ligne de souliers de luxe au service du Cameroun et de l’Afrique de l’Ouest et centrale, avec quinze maisons à travers toutes les silhouettes — baskets aux derbys de ville, mocassins aux sandales à semelle de liège.",
  "about.p2":
    "Livraison sous quarante-huit heures, retours acceptés pendant quatorze jours dans la boîte d’origine, et seules sont mises en vente les paires sourcées directement auprès de la maison ou de son distributeur agréé.",
  "about.cta": "Parcourir la collection",

  // ── WhatsApp ───────────────────────────────────────────────────────────────
  "wa.cta.eyebrow": "Ligne directe",
  "wa.cta.title": "Une question ? Écrivez-nous.",
  "wa.cta.body":
    "Un changement d’adresse, une autre pointure, une question sur votre livraison — en un geste vous parlez à l’atelier, pas à une file d’attente.",
  "wa.cta.button": "Discuter sur WhatsApp",
  "wa.cta.hours": "Tous les jours, 08:00–22:00 WAT",
  "wa.cta.greeting": "Bonjour {{brand}} 👋",
  "wa.cta.with_order":
    "Je viens de passer la commande {{order}} ({{amount}}) et souhaite confirmer ma livraison.",
  "wa.cta.without_order": "J’aimerais de l’aide concernant ma commande, s’il vous plaît.",
  "wa.fab_aria": "Discuter avec nous sur WhatsApp",
  "wa.dialog_aria": "Conciergerie",
  "wa.eyebrow": "Conciergerie",
  "wa.title": "Comment pouvons-nous vous aider ?",
  "wa.hours":
    "Réponse en quelques minutes, tous les jours de 08:00 à 22:00 WAT.",
  "wa.topic.order_tracking": "Suivre une commande",
  "wa.topic.sizing": "Aide pour la pointure",
  "wa.topic.returns": "Commencer un retour",
  "wa.topic.concierge": "Réserver la conciergerie",
  "wa.topic.general": "Autre chose",
  "wa.separator": "— ou écrivez-nous —",
  "wa.placeholder": "Un mot pour la conciergerie…",
  "wa.send": "Envoyer via WhatsApp",
  "wa.msg.header": "Bonjour Happy Feet,",
  "wa.msg.footer_sent_from": "Envoyé depuis happyfeet.com",
  "wa.msg.sizing":
    "J’ai besoin d’aide pour choisir la bonne pointure. La conciergerie peut-elle m’aider ?",
  "wa.msg.concierge":
    "Je souhaite réserver la conciergerie — trois paires choisies pour moi. Je peux envoyer trois photos des souliers que je porte le plus.",
  "wa.msg.order_tracking": "Je souhaite un point sur une commande récente.",
  "wa.msg.returns": "Je souhaite commencer un retour.",
  "wa.msg.support_intro": "J’aurais besoin d’aide pour ceci :",
  "wa.msg.general": "J’ai une question pour la conciergerie.",
};

export const DICT: Record<Locale, Dict> = { en, fr };

export type Vars = Record<string, string | number>;

export function translate(locale: Locale, key: string, vars?: Vars): string {
  const fromLocale = DICT[locale]?.[key];
  const value = fromLocale ?? DICT[DEFAULT_LOCALE][key] ?? key;
  if (!vars) return value;
  return value.replace(/\{\{(\w+)\}\}/g, (_, k: string) =>
    Object.prototype.hasOwnProperty.call(vars, k) ? String(vars[k]) : `{{${k}}}`,
  );
}

export function isLocale(v: unknown): v is Locale {
  return typeof v === "string" && (SUPPORTED_LOCALES as readonly string[]).includes(v);
}
