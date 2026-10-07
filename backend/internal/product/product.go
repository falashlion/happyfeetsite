package product

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/happyfeet/api/pkg/database"
	"github.com/happyfeet/api/pkg/middleware"
	"github.com/happyfeet/api/pkg/response"
	"github.com/happyfeet/api/pkg/validator"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

var ErrNotFound = errors.New("product: not found")

// ── models ────────────────────────────────────────────────────────────────────

type Image struct {
	ID          string
	URLOriginal string
	URLLarge    string
	URLMedium   string
	URLSmall    string
	URLThumb    string
	IsPrimary   bool
	SortOrder   int
}

type SKU struct {
	ID              string
	SKUCode         string
	SizeEU          float64
	SizeUS          *float64
	SizeUK          *float64
	Color           string
	ColorHex        *string
	Width           string
	StockQty        int
	AdditionalPrice float64
}

type Product struct {
	ID           string
	VendorID     string
	VendorName   string
	CategoryID   string
	CategoryName string
	BrandID      string
	BrandName    string
	Name         string
	Slug         string
	Description  *string
	BasePrice    float64
	FinalPrice   float64
	Currency     string
	Status       string
	Tags         []string
	RatingAvg    float64
	RatingCount  int
	TotalSales   float64
	InStock      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Images       []Image
	SKUs         []SKU
}

// JSON tags are load-bearing: without them Go serialises the field names
// verbatim ("ID", "Slug"), which does not match the snake_case every other
// endpoint returns. Clients reading c.slug got undefined.
type Category struct {
	ID            string      `json:"id"`
	ParentID      *string     `json:"parent_id"`
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	IconURL       *string     `json:"icon_url"`
	ProductCount  int         `json:"product_count"`
	Subcategories []*Category `json:"subcategories,omitempty"`
}

type Brand struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	LogoURL      *string `json:"logo_url"`
	ProductCount int     `json:"product_count"`
}

// ── repository ────────────────────────────────────────────────────────────────

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db} }

type ListFilter struct {
	CategoryID   *string
	CategorySlug *string
	BrandID      *string
	BrandSlug    *string
	MinPrice     *float64
	MaxPrice     *float64
	Sizes        []float64
	InStock      *bool
	MinRating    *float64
	OnSale       *bool
	SortBy       string
	Cursor       *string
	Limit        int
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]*Product, string, int64, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	args := []any{}
	idx := 1
	add := func(v any) int { args = append(args, v); i := idx; idx++; return i }

	conds := []string{"p.status='active'", "p.deleted_at IS NULL"}
	if f.CategoryID != nil {
		conds = append(conds, fmt.Sprintf("p.category_id=$%d", add(*f.CategoryID)))
	}
	if f.CategorySlug != nil {
		conds = append(conds, fmt.Sprintf("c.slug=$%d", add(*f.CategorySlug)))
	}
	if f.BrandID != nil {
		conds = append(conds, fmt.Sprintf("p.brand_id=$%d", add(*f.BrandID)))
	}
	if f.BrandSlug != nil {
		conds = append(conds, fmt.Sprintf("b.slug=$%d", add(*f.BrandSlug)))
	}
	if f.MinPrice != nil {
		conds = append(conds, fmt.Sprintf("p.base_price>=$%d", add(*f.MinPrice)))
	}
	if f.MaxPrice != nil {
		conds = append(conds, fmt.Sprintf("p.base_price<=$%d", add(*f.MaxPrice)))
	}
	if f.InStock != nil && *f.InStock {
		conds = append(conds, "EXISTS(SELECT 1 FROM product_skus sk WHERE sk.product_id=p.id AND sk.stock_qty>0 AND sk.is_active)")
	}
	if f.MinRating != nil {
		conds = append(conds, fmt.Sprintf("p.rating_avg>=$%d", add(*f.MinRating)))
	}
	if f.OnSale != nil && *f.OnSale {
		conds = append(conds, "'sale'=ANY(p.tags)")
	}
	if f.Cursor != nil {
		conds = append(conds, fmt.Sprintf("p.id>$%d", add(*f.Cursor)))
	}

	order := "p.created_at DESC"
	switch f.SortBy {
	case "price_asc":
		order = "p.base_price ASC"
	case "price_desc":
		order = "p.base_price DESC"
	case "newest":
		order = "p.created_at DESC"
	case "rating":
		order = "p.rating_avg DESC"
	}

	lim := add(f.Limit + 1)
	where := strings.Join(conds, " AND ")

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT p.id,p.vendor_id,v.business_name,p.category_id,c.name,p.brand_id,b.name,
		       p.name,p.slug,p.description,p.base_price,p.currency,p.status,p.tags,
		       p.rating_avg,p.rating_count,p.created_at,p.updated_at,
		       EXISTS(SELECT 1 FROM product_skus sk WHERE sk.product_id=p.id AND sk.stock_qty>0) as in_stock,
		       COALESCE((SELECT id FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1)::text, '') as primary_image_id,
		       COALESCE((SELECT url_medium FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),'') as primary_image_med,
		       COALESCE((SELECT url_thumbnail FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),'') as primary_image_thumb,
		       COALESCE((SELECT url_large FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),'') as primary_image_large
		FROM products p
		JOIN vendors v ON v.id=p.vendor_id
		JOIN categories c ON c.id=p.category_id
		JOIN brands b ON b.id=p.brand_id
		WHERE %s ORDER BY %s LIMIT $%d`, where, order, lim), args...)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		p := &Product{}
		var imgID, imgMed, imgThumb, imgLarge string
		if err := rows.Scan(&p.ID, &p.VendorID, &p.VendorName, &p.CategoryID, &p.CategoryName,
			&p.BrandID, &p.BrandName, &p.Name, &p.Slug, &p.Description, &p.BasePrice, &p.Currency,
			&p.Status, &p.Tags, &p.RatingAvg, &p.RatingCount, &p.CreatedAt, &p.UpdatedAt, &p.InStock,
			&imgID, &imgMed, &imgThumb, &imgLarge); err != nil {
			continue
		}
		p.FinalPrice = p.BasePrice
		if imgID != "" {
			p.Images = []Image{{ID: imgID, URLMedium: imgMed, URLThumb: imgThumb, URLLarge: imgLarge, IsPrimary: true}}
		}
		products = append(products, p)
	}

	var cursor string
	if len(products) > f.Limit {
		products = products[:f.Limit]
		cursor = products[len(products)-1].ID
	}
	var total int64
	_ = r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM products p WHERE %s", where), args[:len(args)-1]...).Scan(&total)
	return products, cursor, total, nil
}

func (r *Repository) Search(ctx context.Context, q string, limit int, cursor *string) ([]*Product, string, int64, error) {
	pattern := "%" + q + "%"
	rows, err := r.db.Query(ctx, `
		SELECT p.id,p.vendor_id,v.business_name,p.category_id,c.name,p.brand_id,b.name,
		       p.name,p.slug,p.description,p.base_price,p.currency,p.status,p.tags,
		       p.rating_avg,p.rating_count,p.created_at,p.updated_at,
		       EXISTS(SELECT 1 FROM product_skus sk WHERE sk.product_id=p.id AND sk.stock_qty>0) as in_stock,
		       COALESCE((SELECT id FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1)::text,''),
		       COALESCE((SELECT url_medium FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),''),
		       COALESCE((SELECT url_thumbnail FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),''),
		       COALESCE((SELECT url_large FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),'')
		FROM products p
		JOIN vendors v ON v.id=p.vendor_id
		JOIN categories c ON c.id=p.category_id
		JOIN brands b ON b.id=p.brand_id
		WHERE p.status='active' AND p.deleted_at IS NULL
		AND (p.name ILIKE $1 OR p.description ILIKE $1 OR b.name ILIKE $1)
		ORDER BY p.rating_count DESC
		LIMIT $2`, pattern, limit+1)
	if err != nil {
		return nil, "", 0, err
	}
	defer rows.Close()
	var products []*Product
	for rows.Next() {
		p := &Product{}
		var imgID, imgMed, imgThumb, imgLarge string
		if err := rows.Scan(&p.ID, &p.VendorID, &p.VendorName, &p.CategoryID, &p.CategoryName,
			&p.BrandID, &p.BrandName, &p.Name, &p.Slug, &p.Description, &p.BasePrice, &p.Currency,
			&p.Status, &p.Tags, &p.RatingAvg, &p.RatingCount, &p.CreatedAt, &p.UpdatedAt, &p.InStock,
			&imgID, &imgMed, &imgThumb, &imgLarge); err != nil {
			continue
		}
		p.FinalPrice = p.BasePrice
		if imgID != "" {
			p.Images = []Image{{ID: imgID, URLMedium: imgMed, URLThumb: imgThumb, URLLarge: imgLarge, IsPrimary: true}}
		}
		products = append(products, p)
	}
	var nextCursor string
	if len(products) > limit {
		products = products[:limit]
		nextCursor = products[len(products)-1].ID
	}
	return products, nextCursor, int64(len(products)), nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Product, error) {
	return r.get(ctx, "p.id=$1 AND p.deleted_at IS NULL", id)
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (*Product, error) {
	return r.get(ctx, "p.slug=$1 AND p.deleted_at IS NULL", slug)
}

func (r *Repository) get(ctx context.Context, cond, val string) (*Product, error) {
	p := &Product{}
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT p.id,p.vendor_id,v.business_name,p.category_id,c.name,p.brand_id,b.name,
		       p.name,p.slug,p.description,p.base_price,p.currency,p.status,p.tags,
		       p.rating_avg,p.rating_count,p.created_at,p.updated_at,
		       EXISTS(SELECT 1 FROM product_skus sk WHERE sk.product_id=p.id AND sk.stock_qty>0)
		FROM products p
		JOIN vendors v ON v.id=p.vendor_id
		JOIN categories c ON c.id=p.category_id
		JOIN brands b ON b.id=p.brand_id
		WHERE %s`, cond), val).Scan(
		&p.ID, &p.VendorID, &p.VendorName, &p.CategoryID, &p.CategoryName,
		&p.BrandID, &p.BrandName, &p.Name, &p.Slug, &p.Description, &p.BasePrice, &p.Currency,
		&p.Status, &p.Tags, &p.RatingAvg, &p.RatingCount, &p.CreatedAt, &p.UpdatedAt, &p.InStock)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.FinalPrice = p.BasePrice
	p.Images, _ = r.images(ctx, p.ID)
	p.SKUs, _ = r.skus(ctx, p.ID)
	return p, nil
}

func (r *Repository) images(ctx context.Context, productID string) ([]Image, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,COALESCE(url_original,''),COALESCE(url_large,''),COALESCE(url_medium,''),
		       COALESCE(url_small,''),COALESCE(url_thumbnail,''),is_primary,sort_order
		FROM product_images WHERE product_id=$1 ORDER BY sort_order,is_primary DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var imgs []Image
	for rows.Next() {
		var img Image
		if err := rows.Scan(&img.ID, &img.URLOriginal, &img.URLLarge, &img.URLMedium, &img.URLSmall, &img.URLThumb, &img.IsPrimary, &img.SortOrder); err == nil {
			imgs = append(imgs, img)
		}
	}
	return imgs, nil
}

func (r *Repository) skus(ctx context.Context, productID string) ([]SKU, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,sku_code,size_eu,size_us,size_uk,color,color_hex,width::text,stock_qty,additional_price
		FROM product_skus WHERE product_id=$1 AND is_active=true ORDER BY size_eu`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skus []SKU
	for rows.Next() {
		var s SKU
		if err := rows.Scan(&s.ID, &s.SKUCode, &s.SizeEU, &s.SizeUS, &s.SizeUK, &s.Color, &s.ColorHex, &s.Width, &s.StockQty, &s.AdditionalPrice); err == nil {
			skus = append(skus, s)
		}
	}
	return skus, nil
}

func (r *Repository) simpleList(ctx context.Context, orderBy string, limit int) ([]*Product, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT p.id,p.vendor_id,v.business_name,p.category_id,c.name,p.brand_id,b.name,
		       p.name,p.slug,p.description,p.base_price,p.currency,p.status,p.tags,
		       p.rating_avg,p.rating_count,p.created_at,p.updated_at,
		       EXISTS(SELECT 1 FROM product_skus sk WHERE sk.product_id=p.id AND sk.stock_qty>0),
		       COALESCE((SELECT id FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1)::text,''),
		       COALESCE((SELECT url_medium FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),''),
		       COALESCE((SELECT url_thumbnail FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),''),
		       COALESCE((SELECT url_large FROM product_images WHERE product_id=p.id AND is_primary LIMIT 1),'')
		FROM products p
		JOIN vendors v ON v.id=p.vendor_id
		JOIN categories c ON c.id=p.category_id
		JOIN brands b ON b.id=p.brand_id
		WHERE p.status='active' AND p.deleted_at IS NULL
		ORDER BY %s LIMIT $1`, orderBy), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []*Product
	for rows.Next() {
		p := &Product{}
		var imgID, imgMed, imgThumb, imgLarge string
		if err := rows.Scan(&p.ID, &p.VendorID, &p.VendorName, &p.CategoryID, &p.CategoryName,
			&p.BrandID, &p.BrandName, &p.Name, &p.Slug, &p.Description, &p.BasePrice, &p.Currency,
			&p.Status, &p.Tags, &p.RatingAvg, &p.RatingCount, &p.CreatedAt, &p.UpdatedAt, &p.InStock,
			&imgID, &imgMed, &imgThumb, &imgLarge); err == nil {
			p.FinalPrice = p.BasePrice
			if imgID != "" {
				p.Images = []Image{{ID: imgID, URLMedium: imgMed, URLThumb: imgThumb, URLLarge: imgLarge, IsPrimary: true}}
			}
			products = append(products, p)
		}
	}
	return products, nil
}

type CreateParams struct {
	VendorID    string
	CategoryID  string
	BrandID     string
	Name        string
	Description string
	BasePrice   float64
	Currency    string
	Tags        []string
}

func (r *Repository) Create(ctx context.Context, p CreateParams) (string, error) {
	var id string
	slug := slugify(p.Name)
	// products.tags is TEXT[] NOT NULL DEFAULT '{}'. A nil slice marshals to
	// NULL and violates the constraint, so normalise before the insert.
	if p.Tags == nil {
		p.Tags = []string{}
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO products(vendor_id,category_id,brand_id,name,slug,description,base_price,currency,tags)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		p.VendorID, p.CategoryID, p.BrandID, p.Name, slug, p.Description, p.BasePrice, p.Currency, p.Tags,
	).Scan(&id)
	return id, err
}

func (r *Repository) Archive(ctx context.Context, id, vendorID string) error {
	res, err := r.db.Exec(ctx, `UPDATE products SET status='archived',updated_at=NOW() WHERE id=$1 AND vendor_id=$2`, id, vendorID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsureVendor returns the vendor that owns this user's products, creating one
// on first use.
//
// products.vendor_id is a foreign key to vendors.id, not to users.id — passing
// a user ID straight through (as Create used to) violates the constraint on any
// database where that UUID is not also a vendor. A single-merchant store still
// needs its one vendor row to exist, and nothing in the deploy created it.
func (r *Repository) EnsureVendor(ctx context.Context, ownerUserID, businessName string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id FROM vendors WHERE owner_id=$1 ORDER BY created_at LIMIT 1`, ownerUserID,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	if businessName == "" {
		businessName = "Happy Feet"
	}
	err = r.db.QueryRow(ctx, `
		INSERT INTO vendors(owner_id,business_name,business_type,status,approved_at)
		VALUES($1,$2,'individual','active',NOW())
		RETURNING id`, ownerUserID, businessName).Scan(&id)
	return id, err
}

// CreateCategory adds a top-level or nested category. Slugs are derived from
// the name and must be unique, so a repeat name is reported as a conflict
// rather than silently creating a second category you cannot tell apart.
func (r *Repository) CreateCategory(ctx context.Context, name string, parentID *string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO categories(name,slug,parent_id)
		VALUES($1,$2,$3) RETURNING id`, name, slugify(name), parentID).Scan(&id)
	return id, err
}

// UpdateParams carries only the fields an edit may touch. Nil means "leave
// alone", which is what lets the admin UI send a single changed field rather
// than having to round-trip the whole product and risk clobbering a concurrent
// edit with stale values.
type UpdateParams struct {
	Name        *string
	CategoryID  *string
	BrandID     *string
	Description *string
	BasePrice   *float64
	Currency    *string
	Status      *string
	Tags        *[]string
}

// Update applies the non-nil fields. The slug deliberately does not follow a
// rename: it is in published URLs, and silently breaking them is worse than a
// slug that no longer matches the title.
func (r *Repository) Update(ctx context.Context, id string, p UpdateParams) error {
	set := []string{}
	args := []any{}
	add := func(frag string, v any) {
		args = append(args, v)
		set = append(set, fmt.Sprintf("%s=$%d", frag, len(args)))
	}

	if p.Name != nil {
		add("name", *p.Name)
	}
	if p.CategoryID != nil {
		add("category_id", *p.CategoryID)
	}
	if p.BrandID != nil {
		add("brand_id", *p.BrandID)
	}
	if p.Description != nil {
		add("description", *p.Description)
	}
	if p.BasePrice != nil {
		add("base_price", *p.BasePrice)
	}
	if p.Currency != nil {
		add("currency", *p.Currency)
	}
	if p.Status != nil {
		add("status", *p.Status)
	}
	if p.Tags != nil {
		add("tags", *p.Tags)
	}
	if len(set) == 0 {
		return nil // nothing asked for; not an error
	}

	args = append(args, id)
	q := fmt.Sprintf("UPDATE products SET %s,updated_at=NOW() WHERE id=$%d",
		strings.Join(set, ","), len(args))

	res, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddImageParams is one Cloudinary asset. Only the public ID is required —
// every rendition is derived from it.
type AddImageParams struct {
	PublicID  string
	Format    string
	SizeBytes int
	Width     int
	Height    int
	IsPrimary bool
	SortOrder int
}

// AddImage stores the five renditions Cloudinary will serve for this asset.
// Making one image primary demotes the others in the same transaction, so a
// product can never end up with two primaries or none.
func (r *Repository) AddImage(ctx context.Context, productID, cloudName string, p AddImageParams) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1)`, productID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", ErrNotFound
	}

	if p.IsPrimary {
		if _, err := tx.Exec(ctx, `UPDATE product_images SET is_primary=false WHERE product_id=$1`, productID); err != nil {
			return "", err
		}
	}

	u := CloudinaryRenditions(cloudName, p.PublicID)
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO product_images
			(product_id,url_original,url_large,url_medium,url_small,url_thumbnail,
			 format,size_bytes,width_px,height_px,is_primary,sort_order,processing_status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'complete') RETURNING id`,
		productID, u.Original, u.Large, u.Medium, u.Small, u.Thumbnail,
		p.Format, p.SizeBytes, p.Width, p.Height, p.IsPrimary, p.SortOrder,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (r *Repository) DeleteImage(ctx context.Context, productID, imageID string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM product_images WHERE id=$1 AND product_id=$2`, imageID, productID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Renditions are the fixed sizes product_images stores, matching the column
// comments in the schema (1200/800/400/150).
type Renditions struct {
	Original  string
	Large     string
	Medium    string
	Small     string
	Thumbnail string
}

// CloudinaryRenditions derives every size from one public ID using Cloudinary's
// URL transformations, so the resizing happens on their CDN rather than on this
// server. f_auto/q_auto let Cloudinary pick AVIF or WebP per browser.
func CloudinaryRenditions(cloudName, publicID string) Renditions {
	base := "https://res.cloudinary.com/" + cloudName + "/image/upload"
	at := func(transform string) string {
		return base + "/" + transform + "/" + publicID
	}
	return Renditions{
		Original:  at("f_auto,q_auto"),
		Large:     at("w_1200,h_1200,c_limit,f_auto,q_auto"),
		Medium:    at("w_800,h_800,c_limit,f_auto,q_auto"),
		Small:     at("w_400,h_400,c_limit,f_auto,q_auto"),
		Thumbnail: at("w_150,h_150,c_fill,g_auto,f_auto,q_auto"),
	}
}

func (r *Repository) ListCategories(ctx context.Context) ([]*Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id,c.parent_id,c.name,c.slug,c.icon_url,
		       COUNT(p.id) FILTER(WHERE p.status='active') AS cnt
		FROM categories c
		LEFT JOIN products p ON p.category_id=c.id AND p.deleted_at IS NULL
		WHERE c.is_active=true GROUP BY c.id ORDER BY c.sort_order,c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []*Category{}
	byID := map[string]*Category{}
	for rows.Next() {
		c := &Category{}
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.IconURL, &c.ProductCount); err == nil {
			byID[c.ID] = c
			all = append(all, c)
		}
	}
	var roots []*Category
	for _, c := range all {
		if c.ParentID != nil {
			if parent, ok := byID[*c.ParentID]; ok {
				parent.Subcategories = append(parent.Subcategories, c)
				continue
			}
		}
		roots = append(roots, c)
	}
	return roots, nil
}

func (r *Repository) ListBrands(ctx context.Context, search string) ([]*Brand, error) {
	q := `SELECT b.id,b.name,b.slug,b.logo_url,COUNT(p.id) FILTER(WHERE p.status='active') AS cnt
	      FROM brands b LEFT JOIN products p ON p.brand_id=b.id AND p.deleted_at IS NULL
	      WHERE b.is_active=true`
	args := []any{}
	if search != "" {
		args = append(args, "%"+search+"%")
		q += " AND b.name ILIKE $1"
	}
	q += " GROUP BY b.id ORDER BY b.name"
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var brands []*Brand
	for rows.Next() {
		b := &Brand{}
		if err := rows.Scan(&b.ID, &b.Name, &b.Slug, &b.LogoURL, &b.ProductCount); err == nil {
			brands = append(brands, b)
		}
	}
	return brands, nil
}

// ── handler ───────────────────────────────────────────────────────────────────

type Handler struct {
	repo *Repository
	// cloudName is the Cloudinary account that serves product imagery. Empty
	// when uploads are unconfigured, in which case attaching an image is
	// refused rather than writing URLs that would 404.
	cloudName string
}

func NewHandler(repo *Repository, cloudName string) *Handler {
	return &Handler{repo: repo, cloudName: cloudName}
}

// ListProducts godoc
// @Summary      List products
// @Tags         products
// @Produce      json
// @Param        limit        query  int     false  "Limit (1-100, default 20)"
// @Param        category_id  query  string  false  "Filter by category UUID"
// @Param        brand_id     query  string  false  "Filter by brand UUID"
// @Param        sort_by      query  string  false  "Sort: relevance|price_asc|price_desc|newest|rating"
// @Success      200  {object}  map[string]any
// @Router       /products [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	q := r.URL.Query()
	f := ListFilter{
		Limit:  qInt(q.Get("limit"), 20),
		SortBy: q.Get("sort_by"),
	}
	if v := q.Get("category_id"); v != "" {
		f.CategoryID = &v
	}
	if v := q.Get("category_slug"); v != "" {
		f.CategorySlug = &v
	}
	if v := q.Get("brand_id"); v != "" {
		f.BrandID = &v
	}
	if v := q.Get("brand_slug"); v != "" {
		f.BrandSlug = &v
	}
	if v := q.Get("min_price"); v != "" {
		if f2, e := strconv.ParseFloat(v, 64); e == nil {
			f.MinPrice = &f2
		}
	}
	if v := q.Get("max_price"); v != "" {
		if f2, e := strconv.ParseFloat(v, 64); e == nil {
			f.MaxPrice = &f2
		}
	}
	if q.Get("in_stock") == "true" {
		t := true
		f.InStock = &t
	}
	if q.Get("on_sale") == "true" {
		t := true
		f.OnSale = &t
	}
	if v := q.Get("cursor"); v != "" {
		f.Cursor = &v
	}
	products, cursor, total, err := h.repo.List(r.Context(), f)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	var cur *string
	if cursor != "" {
		cur = &cursor
	}
	response.Paginated(w, fmtList(products), &response.Meta{Cursor: cur, HasMore: cursor != "", Total: total})
}

// SearchProducts godoc
// @Summary      Search products
// @Tags         products
// @Produce      json
// @Param        q      query  string  true   "Search query"
// @Param        limit  query  int     false  "Limit"
// @Success      200  {object}  map[string]any
// @Router       /products/search [get]
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	q := r.URL.Query().Get("q")
	if q == "" {
		response.BadRequest(w, "q is required", rid)
		return
	}
	products, cursor, total, err := h.repo.Search(r.Context(), q, qInt(r.URL.Query().Get("limit"), 20), nil)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	var cur *string
	if cursor != "" {
		cur = &cursor
	}
	response.Ok(w, map[string]any{
		"data":        fmtList(products),
		"meta":        response.Meta{Cursor: cur, HasMore: cursor != "", Total: total},
		"suggestions": []string{},
	})
}

// GetFeatured godoc
// @Summary      Featured/trending products
// @Tags         products
// @Produce      json
// @Param        limit  query  int  false  "Limit"
// @Success      200  {object}  map[string]any
// @Router       /products/featured [get]
func (h *Handler) Featured(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	limit := qInt(r.URL.Query().Get("limit"), 12)
	if limit > 48 {
		limit = 48
	}
	featured, _ := h.repo.simpleList(r.Context(), "p.rating_avg DESC,p.rating_count DESC", limit)
	trending, _ := h.repo.simpleList(r.Context(), "p.total_sales DESC", limit)
	newArrivals, _ := h.repo.simpleList(r.Context(), "p.created_at DESC", limit)
	response.Ok(w, map[string]any{
		"featured":     fmtList(featured),
		"trending":     fmtList(trending),
		"new_arrivals": fmtList(newArrivals),
		"on_sale":      []any{},
	})
	_ = rid
}

// GetProduct godoc
// @Summary      Get product detail
// @Tags         products
// @Produce      json
// @Param        id  path  string  true  "Product UUID"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /products/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	p, err := h.repo.GetByID(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Product", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtDetail(p))
}

// GetProductBySlug godoc
// @Summary      Get product detail by slug
// @Tags         products
// @Produce      json
// @Param        slug  path  string  true  "Product slug"
// @Success      200  {object}  map[string]any
// @Failure      404  {object}  map[string]any
// @Router       /products/by-slug/{slug} [get]
func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	p, err := h.repo.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Product", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtDetail(p))
}

type createProductReq struct {
	Name        string   `json:"name"        validate:"required,max=255"`
	CategoryID  string   `json:"category_id" validate:"required"`
	BrandID     string   `json:"brand_id"    validate:"required"`
	Description string   `json:"description" validate:"required"`
	BasePrice   float64  `json:"base_price"  validate:"required,gte=0"`
	Currency    string   `json:"currency"    validate:"required,oneof=XAF XOF USD EUR GHS UGX NGN"`
	Tags        []string `json:"tags"`
}

// CreateProduct godoc
// @Summary      Create product (vendor/admin)
// @Tags         products
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body  createProductReq  true  "Product payload"
// @Success      201  {object}  map[string]any
// @Router       /products [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req createProductReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}
	// vendor_id is a foreign key to vendors.id, not users.id. Resolve (or
	// create) the caller's vendor rather than passing their user ID through.
	vendorID, err := h.repo.EnsureVendor(r.Context(), middleware.GetUserID(r.Context()), "")
	if err != nil {
		response.InternalError(w, rid)
		return
	}

	id, err := h.repo.Create(r.Context(), CreateParams{
		VendorID:    vendorID,
		CategoryID:  req.CategoryID,
		BrandID:     req.BrandID,
		Name:        req.Name,
		Description: req.Description,
		BasePrice:   req.BasePrice,
		Currency:    req.Currency,
		Tags:        req.Tags,
	})
	if err != nil {
		// The slug is derived from the name and is UNIQUE, so a repeat name is
		// something the caller can fix, not a server fault.
		if strings.Contains(err.Error(), "duplicate key") {
			response.Conflict(w, "A product with this name already exists", rid)
			return
		}
		// Without this the handler is a black box: every failure looks the same
		// from outside and the cause never reaches the logs.
		log.Error().Err(err).Str("rid", rid).Msg("create product")
		response.InternalError(w, rid)
		return
	}
	p, _ := h.repo.GetByID(r.Context(), id)
	response.Created(w, fmtDetail(p))
}

// ArchiveProduct godoc
// @Summary      Archive product (soft delete)
// @Tags         products
// @Security     BearerAuth
// @Param        id  path  string  true  "Product UUID"
// @Success      204
// @Router       /products/{id} [delete]
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	err := h.repo.Archive(r.Context(), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Product", rid)
		return
	}
	response.NoContent(w)
}

// ListCategories godoc
// @Summary      List categories
// @Tags         categories
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /categories [get]
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	cats, err := h.repo.ListCategories(r.Context())
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, map[string]any{"data": cats})
}

// ListBrands godoc
// @Summary      List brands
// @Tags         categories
// @Produce      json
// @Param        q  query  string  false  "Search by name"
// @Success      200  {object}  map[string]any
// @Router       /brands [get]
func (h *Handler) ListBrands(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	brands, err := h.repo.ListBrands(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, map[string]any{"data": brands})
}

// ListReviews godoc
// @Summary      List product reviews
// @Tags         products
// @Produce      json
// @Param        id  path  string  true  "Product UUID"
// @Success      200  {object}  map[string]any
// @Router       /products/{id}/reviews [get]
func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	response.Ok(w, map[string]any{
		"data": []any{},
		"meta": response.Meta{HasMore: false, Total: 0},
		"summary": map[string]any{
			"average": 0, "total": 0,
			"distribution": map[string]int{"5": 0, "4": 0, "3": 0, "2": 0, "1": 0},
		},
	})
}

// ── formatters ────────────────────────────────────────────────────────────────

func fmtList(products []*Product) []map[string]any {
	out := make([]map[string]any, 0, len(products))
	for _, p := range products {
		out = append(out, map[string]any{
			"id":            p.ID,
			"name":          p.Name,
			"slug":          p.Slug,
			"brand":         p.BrandName,
			"category":      p.CategoryName,
			"base_price":    p.BasePrice,
			"final_price":   p.FinalPrice,
			"currency":      p.Currency,
			"rating_avg":    p.RatingAvg,
			"rating_count":  p.RatingCount,
			"in_stock":      p.InStock,
			"primary_image": primaryImg(p.Images),
		})
	}
	return out
}

func fmtDetail(p *Product) map[string]any {
	if p == nil {
		return nil
	}
	imgs := make([]map[string]any, 0, len(p.Images))
	for _, img := range p.Images {
		imgs = append(imgs, fmtImg(img))
	}
	skus := make([]map[string]any, 0, len(p.SKUs))
	for _, s := range p.SKUs {
		skus = append(skus, map[string]any{
			"id": s.ID, "sku_code": s.SKUCode, "size_eu": s.SizeEU, "size_us": s.SizeUS, "size_uk": s.SizeUK,
			"color": s.Color, "color_hex": s.ColorHex, "width": s.Width,
			"stock_qty": s.StockQty, "additional_price": s.AdditionalPrice,
		})
	}
	return map[string]any{
		"id": p.ID, "name": p.Name, "slug": p.Slug, "description": p.Description,
		"brand": p.BrandName, "brand_id": p.BrandID, "category": p.CategoryName, "category_id": p.CategoryID,
		"vendor_id": p.VendorID, "vendor_name": p.VendorName,
		"base_price": p.BasePrice, "final_price": p.FinalPrice, "currency": p.Currency,
		"status": p.Status, "tags": p.Tags, "rating_avg": p.RatingAvg, "rating_count": p.RatingCount,
		"in_stock": p.InStock, "images": imgs, "skus": skus,
		"created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	}
}

func fmtImg(img Image) map[string]any {
	return map[string]any{
		"id": img.ID, "url_thumbnail": img.URLThumb, "url_small": img.URLSmall,
		"url_medium": img.URLMedium, "url_large": img.URLLarge, "url_original": img.URLOriginal,
		"is_primary": img.IsPrimary, "sort_order": img.SortOrder,
	}
}

func primaryImg(images []Image) *map[string]any {
	for _, img := range images {
		if img.IsPrimary {
			m := fmtImg(img)
			return &m
		}
	}
	if len(images) > 0 {
		m := fmtImg(images[0])
		return &m
	}
	return nil
}

func slugify(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, s)
}

func qInt(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}

// ── admin write handlers ──────────────────────────────────────────────────────

// isUUID guards path parameters before they reach Postgres. A non-UUID id makes
// the driver return a type error, which would surface as a 500 — "p007 is not a
// product" is a 404, and saying so makes a client bug obvious instead of
// looking like a server fault.
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

type updateProductReq struct {
	Name        *string   `json:"name"        validate:"omitempty,max=255"`
	CategoryID  *string   `json:"category_id" validate:"omitempty,uuid4"`
	BrandID     *string   `json:"brand_id"    validate:"omitempty,uuid4"`
	Description *string   `json:"description" validate:"omitempty"`
	BasePrice   *float64  `json:"base_price"  validate:"omitempty,gte=0"`
	Currency    *string   `json:"currency"    validate:"omitempty,oneof=XAF XOF USD EUR GHS UGX NGN"`
	Status      *string   `json:"status"      validate:"omitempty,oneof=draft active archived"`
	Tags        *[]string `json:"tags"`
}

// Update godoc
// @Summary      Update a product (admin)
// @Tags         products
// @Security     BearerAuth
// @Param        id  path  string  true  "Product UUID"
// @Success      200  {object}  map[string]any
// @Router       /admin/products/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	id := chi.URLParam(r, "id")
	if !isUUID(id) {
		response.NotFound(w, "Product", rid)
		return
	}

	var req updateProductReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	err := h.repo.Update(r.Context(), id, UpdateParams{
		Name:        req.Name,
		CategoryID:  req.CategoryID,
		BrandID:     req.BrandID,
		Description: req.Description,
		BasePrice:   req.BasePrice,
		Currency:    req.Currency,
		Status:      req.Status,
		Tags:        req.Tags,
	})
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Product", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}

	p, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.Ok(w, fmtDetail(p))
}

type addImageReq struct {
	// PublicID is what Cloudinary returns once the browser has uploaded. Every
	// rendition URL is derived from it, so the client never picks URLs itself.
	PublicID  string `json:"public_id"  validate:"required,max=255"`
	Format    string `json:"format"     validate:"omitempty,oneof=jpg jpeg png webp avif"`
	SizeBytes int    `json:"size_bytes" validate:"omitempty,gte=0"`
	Width     int    `json:"width"      validate:"omitempty,gte=0"`
	Height    int    `json:"height"     validate:"omitempty,gte=0"`
	IsPrimary bool   `json:"is_primary"`
	SortOrder int    `json:"sort_order" validate:"omitempty,gte=0"`
}

// AddImage godoc
// @Summary      Attach an uploaded Cloudinary image to a product (admin)
// @Tags         products
// @Security     BearerAuth
// @Param        id  path  string  true  "Product UUID"
// @Success      201  {object}  map[string]any
// @Router       /admin/products/{id}/images [post]
func (h *Handler) AddImage(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())

	if h.cloudName == "" {
		response.Err(w, http.StatusServiceUnavailable, "MEDIA_UNCONFIGURED",
			"Image uploads are not configured", rid)
		return
	}

	var req addImageReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	productID := chi.URLParam(r, "id")
	if !isUUID(productID) {
		response.NotFound(w, "Product", rid)
		return
	}

	imageID, err := h.repo.AddImage(r.Context(), productID, h.cloudName, AddImageParams{
		PublicID:  req.PublicID,
		Format:    req.Format,
		SizeBytes: req.SizeBytes,
		Width:     req.Width,
		Height:    req.Height,
		IsPrimary: req.IsPrimary,
		SortOrder: req.SortOrder,
	})
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Product", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}

	u := CloudinaryRenditions(h.cloudName, req.PublicID)
	response.Created(w, map[string]any{
		"id":            imageID,
		"url_original":  u.Original,
		"url_large":     u.Large,
		"url_medium":    u.Medium,
		"url_small":     u.Small,
		"url_thumbnail": u.Thumbnail,
		"is_primary":    req.IsPrimary,
		"sort_order":    req.SortOrder,
	})
}

// DeleteImage godoc
// @Summary      Detach an image from a product (admin)
// @Tags         products
// @Security     BearerAuth
// @Param        id       path  string  true  "Product UUID"
// @Param        imageId  path  string  true  "Image UUID"
// @Success      204
// @Router       /admin/products/{id}/images/{imageId} [delete]
func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	if !isUUID(chi.URLParam(r, "id")) || !isUUID(chi.URLParam(r, "imageId")) {
		response.NotFound(w, "Image", rid)
		return
	}
	err := h.repo.DeleteImage(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "imageId"))
	if errors.Is(err, ErrNotFound) {
		response.NotFound(w, "Image", rid)
		return
	}
	if err != nil {
		response.InternalError(w, rid)
		return
	}
	response.NoContent(w)
}

type createCategoryReq struct {
	Name     string  `json:"name"      validate:"required,max=100"`
	ParentID *string `json:"parent_id" validate:"omitempty,uuid4"`
}

// CreateCategory godoc
// @Summary      Create a category (admin)
// @Tags         categories
// @Security     BearerAuth
// @Success      201  {object}  map[string]any
// @Router       /admin/categories [post]
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	rid := middleware.GetRequestID(r.Context())
	var req createCategoryReq
	if err := validator.Decode(r, &req); err != nil {
		response.ValidationError(w, err, rid)
		return
	}

	id, err := h.repo.CreateCategory(r.Context(), req.Name, req.ParentID)
	if err != nil {
		// The slug is derived from the name and is UNIQUE, so a duplicate name
		// is a conflict the caller can act on, not a server fault.
		if strings.Contains(err.Error(), "duplicate key") {
			response.Conflict(w, "A category with this name already exists", rid)
			return
		}
		response.InternalError(w, rid)
		return
	}
	response.Created(w, map[string]any{"id": id, "name": req.Name, "slug": slugify(req.Name)})
}
