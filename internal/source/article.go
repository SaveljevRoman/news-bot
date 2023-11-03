package source

import (
	"context"
	"database/sql"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"news-bot/internal/model"
	"time"
)

type ArticlePostgresStorage struct {
	db *sqlx.DB
}

type dbArticle struct {
	ID          int64        `db:"id"`
	SourceId    int64        `db:"source_id"`
	Title       string       `db:"title"`
	Link        string       `db:"link"`
	Summary     string       `db:"summary"`
	PublishedAt time.Time    `db:"published_at"`
	PostedAt    sql.NullTime `db:"posted_at"`
	CreatedAt   time.Time    `db:"created_at"`
}

func NewArticleStorage(db *sqlx.DB) *ArticlePostgresStorage {
	return &ArticlePostgresStorage{db: db}
}

func (a *ArticlePostgresStorage) Store(ctx context.Context, article model.Article) error {
	conn, err := a.connection(ctx)
	if err != nil {
		return err
	}

	if _, err := conn.ExecContext(
		ctx,
		`INSERT INTO articles (source_id, title, link, summary, published_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
		article.SourceID,
		article.Title,
		article.Link,
		article.Summary,
		article.PublishedAt,
	); err != nil {
		return err
	}

	return nil
}

func (a *ArticlePostgresStorage) AllNotPosted(ctx context.Context, since time.Time, limit uint64) ([]model.Article, error) {
	conn, err := a.connection(ctx)
	if err != nil {
		return nil, err
	}

	var articles []dbArticle

	if err := conn.SelectContext(
		ctx,
		&articles,
		`SELECT * FROM articles WHERE posted_at IS NULL AND published_at >= $1::timestamp ORDER BY published_at DESC LIMIT $2`,
		since.UTC().Format(time.RFC3339),
		limit,
	); err != nil {
		return nil, err
	}

	return lo.Map(articles, func(article dbArticle, _ int) model.Article {
		return model.Article{
			ID:          article.ID,
			SourceID:    article.SourceId,
			Title:       article.Title,
			Link:        article.Link,
			Summary:     article.Summary,
			PublishedAt: article.PostedAt.Time,
			PostedAt:    article.PublishedAt,
			CreatedAt:   article.CreatedAt,
		}
	}), nil
}

func (a *ArticlePostgresStorage) MarkPosted(ctx context.Context, id int64) error {
	conn, err := a.connection(ctx)
	if err != nil {
		return err
	}

	if _, err := conn.ExecContext(
		ctx,
		`UPDATE articles SET posted_at = $1::timestamp WHERE id = $2`,
		time.Now().UTC().Format(time.RFC3339),
		id,
	); err != nil {
		return err
	}

	return nil
}

func (a *ArticlePostgresStorage) connection(ctx context.Context) (*sqlx.Conn, error) {
	conn, err := a.db.Connx(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn, nil
}
