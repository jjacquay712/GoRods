/*** Copyright (c) 2016, University of Florida Research Foundation, Inc. ***
 *** For more information please refer to the LICENSE.md file            ***/

// Package gorods is a Golang binding for the iRods C API (iRods client library).
// GoRods uses cgo to call iRods client functions.
package gorods

// #include "wrapper.h"
import "C"

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type Group struct {
	Name       string
	CreateTime time.Time
	ModifyTime time.Time
	Id         int
	Type       int
	Zone       string // Need to convert
	Info       string
	Comment    string
	N          int

	Init bool

	Users Users
	Con   *Connection
}

type Groups []*Group

// initGroup
func initGroup(name string, con *Connection, n int) (*Group, error) {

	grp := new(Group)

	grp.Name = name
	grp.Con = con
	grp.N = n

	return grp, nil
}

func (grp *Group) GetName() string {
	return grp.Name
}

func (grp *Group) GetZone() string {
	grp.init()

	return grp.Zone
}

func (grp *Group) GetComment() string {
	grp.init()
	return grp.Comment
}

func (grp *Group) GetCreateTime() time.Time {
	grp.init()
	return grp.CreateTime
}

func (grp *Group) GetModifyTime() time.Time {
	grp.init()
	return grp.ModifyTime
}

func (grp *Group) GetId() int {
	grp.init()
	return grp.Id
}

func (grp *Group) GetType() int {
	grp.init()
	return grp.Type
}

func (grp *Group) GetCon() *Connection {
	return grp.Con
}

func (grp *Group) GetUsers() (Users, error) {
	if err := grp.init(); err != nil {
		return nil, err
	}

	return grp.Users, nil
}

func (grp *Group) Delete() error {
	if err := DeleteGroup(grp.GetName(), grp.GetZone(), grp.Con); err != nil {
		return err
	}

	if err := grp.Con.RefreshGroups(); err != nil {
		return err
	}

	return nil
}

func (grp *Group) init() error {
	if !grp.Init {
		if err := grp.RefreshInfo(); err != nil {
			return err
		}
		if err := grp.RefreshUsers(); err != nil {
			return err
		}
		grp.Init = true
	}

	return nil
}

func (grps Groups) FindByName(name string) *Group {
	for _, grp := range grps {
		if grp.Name == name {
			return grp
		}
	}
	return nil
}

func (grps *Groups) Remove(index int) {
	*grps = append((*grps)[:index], (*grps)[index+1:]...)
}

func (grp *Group) String() string {
	return fmt.Sprintf("%v", grp.Name)
}

func (grp *Group) RefreshInfo() error {
	// r_comment:
	// create_ts:01471444167
	// modify_ts:01471444167
	// user_id:10019
	// user_name:designers
	// user_type_name:rodsgroup
	// zone_name:tempZone
	// user_info:

	if infoMap, err := grp.FetchInfo(); err == nil {
		grp.Comment = infoMap["r_comment"]
		grp.CreateTime = TimeStringToTime(infoMap["create_ts"])
		grp.ModifyTime = TimeStringToTime(infoMap["modify_ts"])
		grp.Id, _ = strconv.Atoi(infoMap["user_id"])
		grp.Type = GroupType
		grp.Zone = infoMap["zone_name"]
		grp.Info = infoMap["user_info"]
	} else {
		return err
	}

	return nil
}

func (grp *Group) RefreshUsers() error {
	if usrs, err := grp.FetchUsers(); err != nil {
		if len(grp.Users) == 0 {
			grp.Users = usrs
		} else {

			// This is broke.com, need to reindex when removing from slice
			// or pass parent slice to user structs during init, so a User.Remove function can be created

			// loop new, add to old if not found
			for _, u := range usrs {
				if found := grp.Users.FindByName(u.GetName()); found == nil {
					grp.Users = append(grp.Users, u)
				}
			}

			oldCopy := make(Users, len(grp.Users))
			copy(oldCopy, grp.Users)

			// loop old, remove from self if not found in new
			for _, u := range oldCopy {
				if found := usrs.FindByName(u.GetName()); found == nil {
					grp.Users.Remove(u.N)
				}
			}
		}
	} else {
		return err
	}

	return nil
}

func (grp *Group) FetchInfo() (map[string]string, error) {
	var (
		result C.goRodsStringResult_t
		err    *C.char
	)

	result.size = C.int(0)

	cGroup := C.CString(grp.Name)
	defer C.free(unsafe.Pointer(cGroup))

	ccon := grp.Con.GetCcon()

	if status := C.gorods_get_user(cGroup, ccon, &result, &err); status != 0 {
		grp.Con.ReturnCcon(ccon)
		return nil, newError(Fatal, fmt.Sprintf("iRods Get Group Info Failed: %v", C.GoString(err)))
	}

	grp.Con.ReturnCcon(ccon)

	defer C.gorods_free_string_result(&result)

	unsafeArr := unsafe.Pointer(result.strArr)
	arrLen := int(result.size)

	// Convert C array to slice, backed by arr *C.char
	slice := (*[1 << 30]*C.char)(unsafeArr)[:arrLen:arrLen]

	response := make(map[string]string)

	for _, groupInfo := range slice {

		groupAttributes := strings.Split(strings.Trim(C.GoString(groupInfo), " \n"), "\n")

		for _, attr := range groupAttributes {

			split := strings.Split(attr, ": ")

			attrName := split[0]
			attrVal := split[1]

			response[attrName] = attrVal

		}
	}

	return response, nil
}

func (grp *Group) FetchUsers() (Users, error) {

	var (
		result C.goRodsStringResult_t
		err    *C.char
	)

	result.size = C.int(0)

	cGroupName := C.CString(grp.Name)
	defer C.free(unsafe.Pointer(cGroupName))

	ccon := grp.Con.GetCcon()

	if status := C.gorods_get_group(ccon, &result, cGroupName, &err); status != 0 {
		grp.Con.ReturnCcon(ccon)
		return nil, newError(Fatal, fmt.Sprintf("iRods Get Group %v Failed: %v", grp.Name, C.GoString(err)))
	}

	grp.Con.ReturnCcon(ccon)
	defer C.gorods_free_string_result(&result)

	unsafeArr := unsafe.Pointer(result.strArr)
	arrLen := int(result.size)

	// Convert C array to slice, backed by arr *C.char
	slice := (*[1 << 30]*C.char)(unsafeArr)[:arrLen:arrLen]

	if usrs, err := grp.Con.GetUsers(); err == nil {
		response := make(Users, 0)

		for _, userNames := range slice {

			usrFrags := strings.Split(C.GoString(userNames), "#")

			if usr := usrs.FindByName(usrFrags[0]); usr != nil {
				response = append(response, usr)
			} else {
				return nil, newError(Fatal, fmt.Sprintf("iRods FetchUsers Failed: User in response not found in cache"))
			}

		}

		return response, nil
	} else {
		return nil, err
	}

}

func (grp *Group) AddUser(usr interface{}) error {

	switch usr.(type) {
	case string:
		// Need to lookup user by string in cache for zone info

		if usrs, err := grp.Con.GetUsers(); err == nil {
			usrName := usr.(string)

			if existingUsr := usrs.FindByName(usrName); existingUsr != nil {
				zoneName := existingUsr.Zone
				return AddToGroup(usrName, zoneName, grp.Name, grp.Con)
			} else {
				return newError(Fatal, fmt.Sprintf("iRods AddUser Failed: can't find iRODS user by string"))
			}
		} else {
			return err
		}

	case *User:
		aUsr := usr.(*User)
		return AddToGroup(aUsr.Name, aUsr.Zone, grp.Name, aUsr.Con)
	default:
	}

	return newError(Fatal, fmt.Sprintf("iRods AddUser Failed: unknown type passed"))
}

func (grp *Group) RemoveUser(usr interface{}) error {
	switch usr.(type) {
	case string:
		// Need to lookup user by string in cache for zone info

		if usrs, err := grp.Con.GetUsers(); err == nil {
			usrName := usr.(string)

			if existingUsr := usrs.FindByName(usrName); existingUsr != nil {
				zoneName := existingUsr.Zone
				return RemoveFromGroup(usrName, zoneName, grp.Name, grp.Con)
			} else {
				return newError(Fatal, fmt.Sprintf("iRods RemoveUser Failed: can't find iRODS user by string"))
			}
		} else {
			return err
		}

	case *User:
		aUsr := usr.(*User)
		return RemoveFromGroup(aUsr.Name, aUsr.Zone, grp.Name, aUsr.Con)
	default:
	}

	return newError(Fatal, fmt.Sprintf("iRods RemoveUser Failed: unknown type passed"))
}

func AddToGroup(userName string, zoneName string, groupName string, con *Connection) error {

	var (
		err *C.char
	)

	cUserName := C.CString(userName)
	cZoneName := C.CString(zoneName)
	cGroupName := C.CString(groupName)
	defer C.free(unsafe.Pointer(cUserName))
	defer C.free(unsafe.Pointer(cZoneName))
	defer C.free(unsafe.Pointer(cGroupName))

	ccon := con.GetCcon()
	defer con.ReturnCcon(ccon)

	if status := C.gorods_add_user_to_group(cUserName, cZoneName, cGroupName, ccon, &err); status != 0 {
		return newError(Fatal, fmt.Sprintf("iRods AddToGroup %v Failed: %v", groupName, C.GoString(err)))
	}

	return nil
}

func RemoveFromGroup(userName string, zoneName string, groupName string, con *Connection) error {
	var (
		err *C.char
	)

	cUserName := C.CString(userName)
	cZoneName := C.CString(zoneName)
	cGroupName := C.CString(groupName)
	defer C.free(unsafe.Pointer(cUserName))
	defer C.free(unsafe.Pointer(cZoneName))
	defer C.free(unsafe.Pointer(cGroupName))

	ccon := con.GetCcon()
	defer con.ReturnCcon(ccon)

	if status := C.gorods_remove_user_from_group(cUserName, cZoneName, cGroupName, ccon, &err); status != 0 {
		return newError(Fatal, fmt.Sprintf("iRods AddToGroup %v Failed: %v", groupName, C.GoString(err)))
	}

	return nil
}

func DeleteGroup(groupName string, zoneName string, con *Connection) error {
	var (
		err *C.char
	)

	cZoneName := C.CString(zoneName)
	cGroupName := C.CString(groupName)
	defer C.free(unsafe.Pointer(cZoneName))
	defer C.free(unsafe.Pointer(cGroupName))

	ccon := con.GetCcon()
	defer con.ReturnCcon(ccon)

	if status := C.gorods_delete_group(cGroupName, cZoneName, ccon, &err); status != 0 {
		return newError(Fatal, fmt.Sprintf("iRods DeleteGroup %v Failed: %v", groupName, C.GoString(err)))
	}

	return nil
}

func CreateGroup(groupName string, zoneName string, con *Connection) error {
	var (
		err *C.char
	)

	cZoneName := C.CString(zoneName)
	cGroupName := C.CString(groupName)
	defer C.free(unsafe.Pointer(cZoneName))
	defer C.free(unsafe.Pointer(cGroupName))

	ccon := con.GetCcon()
	defer con.ReturnCcon(ccon)

	if status := C.gorods_create_group(cGroupName, cZoneName, ccon, &err); status != 0 {
		return newError(Fatal, fmt.Sprintf("iRods CreateGroup %v Failed: %v", groupName, C.GoString(err)))
	}

	return nil
}
