package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// KnowledgeRepository is the SQLite-backed implementation of
// knowledge.Repository.
type KnowledgeRepository struct {
	db *sql.DB
}

// NewKnowledgeRepository creates a KnowledgeRepository backed by db. db
// must already have its migrations applied (see Open).
func NewKnowledgeRepository(db *sql.DB) *KnowledgeRepository {
	return &KnowledgeRepository{db: db}
}

const knowledgeItemColumns = `id, topic, concept, definition, properties, trade_offs, related_concepts, source, status, created_at, updated_at`

// knowledgeItemSelectColumns reads the three JSON-array-as-TEXT columns
// through COALESCE so a NULL value (e.g. a pre-existing row from before
// these columns existed) decodes as "" rather than failing the Scan into
// a plain string. unmarshalStringList treats "" as an empty list.
const knowledgeItemSelectColumns = `id, topic, concept, definition, COALESCE(properties, ''), COALESCE(trade_offs, ''), COALESCE(related_concepts, ''), source, status, created_at, updated_at`

// Save inserts a new knowledge item.
func (r *KnowledgeRepository) Save(ctx context.Context, item knowledge.Item) error {
	properties, tradeOffs, relatedConcepts, err := marshalItemLists(item)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO knowledge_items (`+knowledgeItemColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Topic, item.Concept, item.Definition,
		properties, tradeOffs, relatedConcepts,
		item.Source, item.Status, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving knowledge item: %w", err)
	}
	return nil
}

// marshalItemLists encodes item's three JSON-array-as-TEXT columns
// together so Save and Update share one error-handling path.
func marshalItemLists(item knowledge.Item) (properties, tradeOffs, relatedConcepts string, err error) {
	properties, err = marshalStringList(item.Properties)
	if err != nil {
		return "", "", "", fmt.Errorf("sqlite: encoding properties for item %s: %w", item.ID, err)
	}
	tradeOffs, err = marshalStringList(item.TradeOffs)
	if err != nil {
		return "", "", "", fmt.Errorf("sqlite: encoding trade_offs for item %s: %w", item.ID, err)
	}
	relatedConcepts, err = marshalStringList(item.RelatedConcepts)
	if err != nil {
		return "", "", "", fmt.Errorf("sqlite: encoding related_concepts for item %s: %w", item.ID, err)
	}
	return properties, tradeOffs, relatedConcepts, nil
}

// GetByID returns the item with the given id, or knowledge.ErrItemNotFound
// if it does not exist.
func (r *KnowledgeRepository) GetByID(ctx context.Context, id string) (knowledge.Item, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+knowledgeItemSelectColumns+` FROM knowledge_items WHERE id = ?`, id,
	)
	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return knowledge.Item{}, knowledge.ErrItemNotFound
	}
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: getting knowledge item: %w", err)
	}
	return item, nil
}

// FindByTopic returns every item for topic, oldest first.
func (r *KnowledgeRepository) FindByTopic(ctx context.Context, topic string) ([]knowledge.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+knowledgeItemSelectColumns+` FROM knowledge_items WHERE topic = ? ORDER BY created_at ASC, id ASC`,
		topic,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: finding knowledge items by topic: %w", err)
	}
	return scanItems(rows)
}

// List returns every item matching filter, oldest first.
func (r *KnowledgeRepository) List(ctx context.Context, filter knowledge.Filter) ([]knowledge.Item, error) {
	var conditions []string
	var args []any
	if filter.Topic != "" {
		conditions = append(conditions, "topic = ?")
		args = append(args, filter.Topic)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}

	queryParts := []string{`SELECT ` + knowledgeItemSelectColumns + ` FROM knowledge_items`}
	if len(conditions) > 0 {
		queryParts = append(queryParts, "WHERE "+strings.Join(conditions, " AND "))
	}
	queryParts = append(queryParts, "ORDER BY created_at ASC, id ASC")
	query := strings.Join(queryParts, " ")

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing knowledge items: %w", err)
	}
	return scanItems(rows)
}

// ListTopics returns every distinct topic, alphabetically.
func (r *KnowledgeRepository) ListTopics(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT topic FROM knowledge_items ORDER BY topic ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing knowledge topics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	topics := []string{}
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, fmt.Errorf("sqlite: scanning knowledge topic: %w", err)
		}
		topics = append(topics, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating knowledge topics: %w", err)
	}
	return topics, nil
}

// CountByStatus returns how many items currently have the given status.
func (r *KnowledgeRepository) CountByStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM knowledge_items WHERE status = ?`, status,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: counting knowledge items by status: %w", err)
	}
	return count, nil
}

// Update persists every field of item, or returns knowledge.ErrItemNotFound
// if it does not exist.
func (r *KnowledgeRepository) Update(ctx context.Context, item knowledge.Item) error {
	properties, tradeOffs, relatedConcepts, err := marshalItemLists(item)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE knowledge_items SET topic = ?, concept = ?, definition = ?, properties = ?, trade_offs = ?, related_concepts = ?, source = ?, status = ?, created_at = ?, updated_at = ? WHERE id = ?`,
		item.Topic, item.Concept, item.Definition,
		properties, tradeOffs, relatedConcepts,
		item.Source, item.Status, item.CreatedAt, item.UpdatedAt, item.ID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: updating knowledge item: %w", err)
	}
	return requireRowAffected(result, knowledge.ErrItemNotFound)
}

// Delete permanently removes the item with the given id, or returns
// knowledge.ErrItemNotFound if it does not exist.
func (r *KnowledgeRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM knowledge_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting knowledge item: %w", err)
	}
	return requireRowAffected(result, knowledge.ErrItemNotFound)
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(scanner rowScanner) (knowledge.Item, error) {
	var item knowledge.Item
	var properties, tradeOffs, relatedConcepts string
	err := scanner.Scan(
		&item.ID, &item.Topic, &item.Concept, &item.Definition,
		&properties, &tradeOffs, &relatedConcepts,
		&item.Source, &item.Status, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return knowledge.Item{}, err
	}

	item.Properties, err = unmarshalStringList(properties)
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: decoding properties for item %s: %w", item.ID, err)
	}
	item.TradeOffs, err = unmarshalStringList(tradeOffs)
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: decoding trade_offs for item %s: %w", item.ID, err)
	}
	item.RelatedConcepts, err = unmarshalStringList(relatedConcepts)
	if err != nil {
		return knowledge.Item{}, fmt.Errorf("sqlite: decoding related_concepts for item %s: %w", item.ID, err)
	}
	return item, nil
}

func scanItems(rows *sql.Rows) ([]knowledge.Item, error) {
	defer func() { _ = rows.Close() }()

	items := []knowledge.Item{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning knowledge item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating knowledge items: %w", err)
	}
	return items, nil
}
