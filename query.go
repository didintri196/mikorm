package mikorm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Query builds and runs commands against one RouterOS menu, such as
// "/ip/address". Query values are immutable: every builder method returns a
// new Query, so a Query can be shared and reused safely.
//
// Where conditions and Select only apply to read operations (Find, First,
// Count and IDs).
type Query struct {
	client *Client
	path   string
	conds  []string
	props  []string
	err    error
}

// Query returns a Query for the given menu path. Both API style
// ("/ip/address") and CLI style ("/ip address") paths are accepted.
func (c *Client) Query(path string) *Query {
	return &Query{client: c, path: normalizePath(path)}
}

func normalizePath(path string) string {
	parts := strings.Fields(strings.ReplaceAll(path, "/", " "))
	return "/" + strings.Join(parts, "/")
}

// Path returns the menu path of the query.
func (q *Query) Path() string { return q.path }

func (q *Query) with(words ...string) *Query {
	n := *q
	n.conds = append(q.conds[:len(q.conds):len(q.conds)], words...)
	return &n
}

func (q *Query) withErr(err error) *Query {
	n := *q
	if n.err == nil {
		n.err = err
	}
	return &n
}

// Where adds the condition "key equals value".
func (q *Query) Where(key string, value any) *Query {
	s, err := formatAny(value)
	if err != nil {
		return q.withErr(fmt.Errorf("mikorm: where %q: %w", key, err))
	}
	return q.with("?" + key + "=" + s)
}

// WhereNot adds the condition "key does not equal value".
func (q *Query) WhereNot(key string, value any) *Query {
	return q.Where(key, value).with("?#!")
}

// WhereIn adds the condition "key equals any of values". With no values
// nothing matches.
func (q *Query) WhereIn(key string, values ...any) *Query {
	if len(values) == 0 {
		// Every record has an .id, so this never matches.
		return q.with("?-.id")
	}
	words := make([]string, 0, len(values)+1)
	for _, v := range values {
		s, err := formatAny(v)
		if err != nil {
			return q.withErr(fmt.Errorf("mikorm: where %q: %w", key, err))
		}
		words = append(words, "?"+key+"="+s)
	}
	if len(values) > 1 {
		words = append(words, "?#"+strings.Repeat("|", len(values)-1))
	}
	return q.with(words...)
}

// WhereExists adds the condition "key is present".
func (q *Query) WhereExists(key string) *Query {
	return q.with("?" + key)
}

// WhereNotExists adds the condition "key is absent".
func (q *Query) WhereNotExists(key string) *Query {
	return q.with("?-" + key)
}

// WhereLess adds the condition "key is less than value".
func (q *Query) WhereLess(key string, value any) *Query {
	s, err := formatAny(value)
	if err != nil {
		return q.withErr(fmt.Errorf("mikorm: where %q: %w", key, err))
	}
	return q.with("?<" + key + "=" + s)
}

// WhereGreater adds the condition "key is greater than value".
func (q *Query) WhereGreater(key string, value any) *Query {
	s, err := formatAny(value)
	if err != nil {
		return q.withErr(fmt.Errorf("mikorm: where %q: %w", key, err))
	}
	return q.with("?>" + key + "=" + s)
}

// WhereModel adds an equality condition for every non-zero field of model,
// which may be a struct (or pointer to struct) or a map.
func (q *Query) WhereModel(model any) *Query {
	pairs, err := encodeValues(model, false)
	if err != nil {
		return q.withErr(err)
	}
	words := make([]string, len(pairs))
	for i, p := range pairs {
		words[i] = "?" + p.key + "=" + p.value
	}
	return q.with(words...)
}

// Select limits the properties returned by the device (.proplist).
func (q *Query) Select(props ...string) *Query {
	n := *q
	n.props = append(q.props[:len(q.props):len(q.props)], props...)
	return &n
}

func (q *Query) printWords(extra ...string) []string {
	words := []string{q.path + "/print"}
	if len(q.props) > 0 {
		words = append(words, "=.proplist="+strings.Join(q.props, ","))
	}
	words = append(words, extra...)
	return append(words, q.conds...)
}

// All runs the query and returns the raw records.
func (q *Query) All(ctx context.Context) ([]map[string]string, error) {
	if q.err != nil {
		return nil, q.err
	}
	reply, err := q.client.run(ctx, q.printWords())
	if err != nil {
		return nil, err
	}
	return reply.Records, nil
}

// Find runs the query and stores every matching record in dest, which must
// be a pointer to a slice of structs, pointers to structs or
// map[string]string.
func (q *Query) Find(ctx context.Context, dest any) error {
	if err := checkSliceDest(dest); err != nil {
		return err
	}
	records, err := q.All(ctx)
	if err != nil {
		return err
	}
	return decodeRecords(records, dest)
}

// First stores the first matching record in dest, which must be a pointer
// to a struct or to a map[string]string. It returns ErrNotFound when no
// record matches.
func (q *Query) First(ctx context.Context, dest any) error {
	if err := checkRecordDest(dest); err != nil {
		return err
	}
	records, err := q.All(ctx)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return ErrNotFound
	}
	return decodeRecord(records[0], dest)
}

// Count returns the number of matching records.
func (q *Query) Count(ctx context.Context) (int, error) {
	if q.err != nil {
		return 0, q.err
	}
	n := *q
	n.props = nil
	reply, err := q.client.run(ctx, n.printWords("=count-only="))
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(reply.Done["ret"])
	if err != nil {
		return 0, fmt.Errorf("mikorm: %s/print: invalid count %q", q.path, reply.Done["ret"])
	}
	return count, nil
}

// IDs returns the .id of every matching record.
func (q *Query) IDs(ctx context.Context) ([]string, error) {
	n := *q
	n.props = []string{".id"}
	records, err := n.All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(records))
	for _, r := range records {
		if id := r[".id"]; id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// Add creates a record from values (a struct, pointer to struct or map) and
// returns its .id. Zero and read-only struct fields are not sent.
func (q *Query) Add(ctx context.Context, values any) (string, error) {
	reply, err := q.Exec(ctx, "add", values)
	if err != nil {
		return "", err
	}
	return reply.Done["ret"], nil
}

// Set updates the record with the given .id using values (a struct,
// pointer to struct or map). Zero and read-only struct fields are not sent.
func (q *Query) Set(ctx context.Context, id string, values any) error {
	if id == "" {
		return errors.New("mikorm: set: id is required")
	}
	return q.execIDs(ctx, "set", []string{id}, values)
}

// Remove deletes the records with the given ids. It does nothing when ids
// is empty.
func (q *Query) Remove(ctx context.Context, ids ...string) error {
	return q.execIDs(ctx, "remove", ids, nil)
}

// Enable enables the records with the given ids.
func (q *Query) Enable(ctx context.Context, ids ...string) error {
	return q.execIDs(ctx, "enable", ids, nil)
}

// Disable disables the records with the given ids.
func (q *Query) Disable(ctx context.Context, ids ...string) error {
	return q.execIDs(ctx, "disable", ids, nil)
}

func (q *Query) execIDs(ctx context.Context, command string, ids []string, values any) error {
	if len(ids) == 0 {
		return nil
	}
	pairs, err := encodeValues(values, true)
	if err != nil {
		return err
	}
	words := []string{q.path + "/" + command, "=.id=" + strings.Join(ids, ",")}
	for _, p := range pairs {
		words = append(words, "="+p.key+"="+p.value)
	}
	_, err = q.client.run(ctx, words)
	return err
}

// Exec runs an arbitrary command of the menu, e.g. Exec(ctx, "add", v)
// on "/ip/address" sends "/ip/address/add". values (a struct, pointer to
// struct, map or nil) are sent as command arguments.
func (q *Query) Exec(ctx context.Context, command string, values any) (*Reply, error) {
	if q.err != nil {
		return nil, q.err
	}
	pairs, err := encodeValues(values, true)
	if err != nil {
		return nil, err
	}
	words := make([]string, 0, len(pairs)+1)
	words = append(words, q.path+"/"+strings.Trim(command, "/"))
	for _, p := range pairs {
		words = append(words, "="+p.key+"="+p.value)
	}
	return q.client.run(ctx, words)
}
