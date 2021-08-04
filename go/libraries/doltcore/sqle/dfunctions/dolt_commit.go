// Copyright 2020 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dfunctions

import (
	"fmt"

	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/vitess/go/vt/proto/query"

	"github.com/dolthub/dolt/go/cmd/dolt/cli"
	"github.com/dolthub/dolt/go/libraries/doltcore/env/actions"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle/dsess"
)

const DoltCommitFuncName = "dolt_commit"

var hashType = sql.MustCreateString(query.Type_TEXT, 32, sql.Collation_ascii_bin)

type DoltCommitFunc struct {
	children []sql.Expression
}

// NewDoltCommitFunc creates a new DoltCommitFunc expression whose children represents the args passed in DOLT_COMMIT.
func NewDoltCommitFunc(ctx *sql.Context, args ...sql.Expression) (sql.Expression, error) {
	return &DoltCommitFunc{children: args}, nil
}

// Runs DOLT_COMMIT in the sql engine which models the behavior of `dolt commit`. Commits staged staged changes to head.
func (d DoltCommitFunc) Eval(ctx *sql.Context, row sql.Row) (interface{}, error) {
	// Get the information for the sql context.
	dbName := ctx.GetCurrentDatabase()
	ap := cli.CreateCommitArgParser()

	// Get the args for DOLT_COMMIT.
	args, err := getDoltArgs(ctx, row, d.Children())
	if err != nil {
		return nil, err
	}

	apr, err := ap.Parse(args)
	if err != nil {
		return nil, err
	}

	dSess := dsess.DSessFromSess(ctx.Session)
	roots, ok := dSess.GetRoots(ctx, dbName)
	if !ok {
		return nil, fmt.Errorf("Could not load database %s", dbName)
	}

	if apr.Contains(cli.AllFlag) {
		roots, err = actions.StageAllTablesNoDocs(ctx, roots)
		if err != nil {
			return nil, fmt.Errorf(err.Error())
		}
	}

	var name, email string
	if authorStr, ok := apr.GetValue(cli.AuthorParam); ok {
		name, email, err = cli.ParseAuthor(authorStr)
		if err != nil {
			return nil, err
		}
	} else {
		name = dSess.Username
		email = dSess.Email
	}

	// Get the commit message.
	msg, msgOk := apr.GetValue(cli.CommitMessageArg)
	if !msgOk {
		return nil, fmt.Errorf("Must provide commit message.")
	}

	t := ctx.QueryTime()
	if commitTimeStr, ok := apr.GetValue(cli.DateParam); ok {
		var err error
		t, err = cli.ParseDate(commitTimeStr)

		if err != nil {
			return nil, fmt.Errorf(err.Error())
		}
	}

	// Commit any pending transaction before a dolt_commit
	tx := ctx.Session.GetTransaction()
	_, ok = tx.(*dsess.DoltTransaction)
	if !ok {
		return nil, fmt.Errorf("expected a DoltTransaction, got %T", tx)
	}

	err = dSess.SetRoots(ctx, dbName, roots)
	if err != nil {
		return nil, err
	}

	err = dSess.CommitTransaction(ctx, dbName, tx)
	if err != nil {
		return nil, err
	}

	// Unsetting the transaction here ensures that it won't be re-committed when this statement concludes
	ctx.SetTransaction(nil)

	var mergeParentCommits []*doltdb.Commit
	ws, err := dSess.WorkingSet(ctx, dbName)
	if err != nil {
		return nil, err
	}

	if ws.MergeActive() {
		mergeParentCommits = []*doltdb.Commit{ws.MergeState().Commit()}
	}

	// Now do a Dolt commit
	commit, err := dSess.CommitToDolt(ctx, roots, mergeParentCommits, dbName, actions.CommitStagedProps{
		Message:    msg,
		Date:       t,
		AllowEmpty: apr.Contains(cli.AllowEmptyFlag),
		Force:      apr.Contains(cli.ForceFlag),
		Name:       name,
		Email:      email,
	})
	if err != nil {
		return 1, err
	}

	cmHash, err := commit.HashOf()
	if err != nil {
		return nil, err
	}

	return cmHash.String(), nil
}

func getDoltArgs(ctx *sql.Context, row sql.Row, children []sql.Expression) ([]string, error) {
	args := make([]string, len(children))
	for i := range children {
		childVal, err := children[i].Eval(ctx, row)

		if err != nil {
			return nil, err
		}

		text, err := sql.Text.Convert(childVal)

		if err != nil {
			return nil, err
		}

		args[i] = text.(string)
	}

	return args, nil
}

func (d DoltCommitFunc) String() string {
	childrenStrings := make([]string, len(d.children))

	for _, child := range d.children {
		childrenStrings = append(childrenStrings, child.String())
	}
	return fmt.Sprintf("commit_hash")
}

func (d DoltCommitFunc) Type() sql.Type {
	return sql.Text
}

func (d DoltCommitFunc) IsNullable() bool {
	return false
}

func (d DoltCommitFunc) WithChildren(ctx *sql.Context, children ...sql.Expression) (sql.Expression, error) {
	return NewDoltCommitFunc(ctx, children...)
}

func (d DoltCommitFunc) Resolved() bool {
	for _, child := range d.Children() {
		if !child.Resolved() {
			return false
		}
	}
	return true
}

func (d DoltCommitFunc) Children() []sql.Expression {
	return d.children
}
