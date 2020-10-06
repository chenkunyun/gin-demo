package service

import (
	"gorm.io/gorm"
	"time"
)

type user struct {
	Id   uint64 `json:"id"`
	Name string `json:"name"`
}

type employee struct {
	EmpNo     uint64 `gorm:"primaryKey"`
	BirthDate time.Time
	FirstName string
	LastName  string
	Gender    string
	HireDate  time.Time
}

func (employee) TableName() string {
	return "employees"
}

type UserService struct {
	Database *gorm.DB
}

func (service *UserService) UserList(count int) (data interface{}, err error) {
	var employees []employee
	find := service.Database.Limit(count).Find(&employees)
	if find.Error != nil {
		return nil, find.Error
	}

	userCount := find.RowsAffected
	users := make([]user, userCount)
	var i int64 = 0
	for ; i < userCount; i++ {
		users[i] = user{Id: employees[i].EmpNo, Name: employees[i].FirstName}
	}

	return users, nil
}
