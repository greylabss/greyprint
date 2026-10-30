package plan

import (
	"fmt"
	"regexp"
)

type Options struct {
	Name     string `json:"-"`
	Module   string `json:"-"`
	Dir      string `json:"-"`
	Router   string `json:"router"`
	Database string `json:"db"`
	Access   string `json:"access"`
	Redis    bool   `json:"redis"`
	Greyship bool   `json:"greyship"`
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func (o Options) Validate() error {
	if !namePattern.MatchString(o.Name) {
		return fmt.Errorf("invalid project name %q: use lowercase letters, digits, - or _, starting with a letter and at most 64 characters", o.Name)
	}
	if o.Module == "" {
		return fmt.Errorf("module path is required")
	}
	if o.Router != "stdlib" && o.Router != "chi" && o.Router != "gin" && o.Router != "echo" && o.Router != "fiber" {
		return fmt.Errorf("unknown --router %q (valid: stdlib, chi, gin, echo, fiber)", o.Router)
	}
	if o.Database != "none" && o.Database != "postgres" && o.Database != "mysql" && o.Database != "sqlite" {
		return fmt.Errorf("unknown --db %q (valid: none, postgres, mysql, sqlite)", o.Database)
	}
	if o.Access != "driver" && o.Access != "sqlc" && o.Access != "gorm" {
		return fmt.Errorf("unknown --access %q (valid: driver, sqlc, gorm)", o.Access)
	}
	if o.Database == "none" && o.Access != "driver" {
		return fmt.Errorf("--access %q requires a database; set --db to postgres, mysql, or sqlite", o.Access)
	}
	return nil
}
