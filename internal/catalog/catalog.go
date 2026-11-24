package catalog

import (
	"encoding/json"
	"fmt"
)

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
	Pages   []uint32 `json:"pages"`
}

type Catalog struct {
	Tables map[string]*Table `json:"tables"`
}

func New() *Catalog                        { return &Catalog{Tables: make(map[string]*Table)} }
func (c *Catalog) Encode() ([]byte, error) { return json.Marshal(c) }
func (c *Catalog) Decode(data []byte) error {
	*c = Catalog{Tables: make(map[string]*Table)}
	return json.Unmarshal(data, c)
}
func (c *Catalog) CreateTable(name string, columns []Column) error {
	if name == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	if _, exists := c.Tables[name]; exists {
		return fmt.Errorf("table %q already exists", name)
	}
	c.Tables[name] = &Table{Name: name, Columns: columns}
	return nil
}
func (c *Catalog) DropTable(name string) error {
	if _, exists := c.Tables[name]; !exists {
		return fmt.Errorf("table %q does not exist", name)
	}
	delete(c.Tables, name)
	return nil
}
