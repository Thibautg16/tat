package user

import (
	"strconv"

	"github.com/ovh/tat"
	"github.com/ovh/tat/tatcli/internal"
	"github.com/spf13/cobra"
)

var cmdUserConvertToSystem = &cobra.Command{
	Use:   "convert",
	Short: "Convert a user to a system user (admin only): tatcli user convert <username> <canListUsersAsAdmin>",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 2 {
			canListUsersAsAdmin, err := strconv.ParseBool(args[1])
			internal.Check(err)
			out, err := internal.Client().UserConvertToSystem(tat.ConvertUserJSON{
				Username:            args[0],
				CanListUsersAsAdmin: canListUsersAsAdmin,
			})
			internal.Check(err)
			internal.Print(out)
		} else {
			internal.Exit("Invalid argument : tatcli user convert --help\n")
		}
	},
}
