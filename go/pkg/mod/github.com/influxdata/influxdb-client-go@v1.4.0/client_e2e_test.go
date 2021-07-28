// +build e2e

// Copyright 2020 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package influxdb2_test

import (
	"context"
	"flag"
	"fmt"
	"github.com/influxdata/influxdb-client-go/api/write"
	"strconv"
	"strings"
	"testing"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go"
	"github.com/influxdata/influxdb-client-go/api"
	"github.com/influxdata/influxdb-client-go/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var authToken string

func init() {
	flag.StringVar(&authToken, "token", "", "authentication token")
}

func TestReady(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", "")

	ok, err := client.Ready(context.Background())
	if err != nil {
		t.Error(err)
	}
	if !ok {
		t.Fail()
	}
}

func TestHealth(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", "")

	health, err := client.Health(context.Background())
	if err != nil {
		t.Error(err)
	}
	require.NotNil(t, health)
	assert.Equal(t, domain.HealthCheckStatusPass, health.Status)
}

func TestWrite(t *testing.T) {
	client := influxdb2.NewClientWithOptions("http://localhost:9999", authToken, influxdb2.DefaultOptions().SetLogLevel(3))
	writeAPI := client.WriteAPI("my-org", "my-bucket")
	errCh := writeAPI.Errors()
	errorsCount := 0
	go func() {
		for err := range errCh {
			errorsCount++
			fmt.Println("Write error: ", err.Error())
		}
	}()
	timestamp := time.Now()
	for i, f := 0, 3.3; i < 10; i++ {
		writeAPI.WriteRecord(fmt.Sprintf("test,a=%d,b=local f=%.2f,i=%di %d", i%2, f, i, timestamp.UnixNano()))
		//writeAPI.Flush()
		f += 3.3
		timestamp = timestamp.Add(time.Nanosecond)
	}

	for i, f := int64(10), 33.0; i < 20; i++ {
		p := influxdb2.NewPoint("test",
			map[string]string{"a": strconv.FormatInt(i%2, 10), "b": "static"},
			map[string]interface{}{"f": f, "i": i},
			timestamp)
		writeAPI.WritePoint(p)
		f += 3.3
		timestamp = timestamp.Add(time.Nanosecond)
	}

	err := client.WriteAPIBlocking("my-org", "my-bucket").WritePoint(context.Background(), influxdb2.NewPointWithMeasurement("test").
		AddTag("a", "3").AddField("i", 20).AddField("f", 4.4))
	assert.Nil(t, err)

	client.Close()
	assert.Equal(t, 0, errorsCount)

}

func TestQueryRaw(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", authToken)

	queryAPI := client.QueryAPI("my-org")
	res, err := queryAPI.QueryRaw(context.Background(), `from(bucket:"my-bucket")|> range(start: -24h) |> filter(fn: (r) => r._measurement == "test")`, influxdb2.DefaultDialect())
	if err != nil {
		t.Error(err)
	} else {
		fmt.Println("QueryResult:")
		fmt.Println(res)
	}
}

func TestQuery(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", authToken)

	queryAPI := client.QueryAPI("my-org")
	fmt.Println("QueryResult")
	result, err := queryAPI.Query(context.Background(), `from(bucket:"my-bucket")|> range(start: -24h) |> filter(fn: (r) => r._measurement == "test")`)
	if err != nil {
		t.Error(err)
	} else {
		for result.Next() {
			if result.TableChanged() {
				fmt.Printf("table: %s\n", result.TableMetadata().String())
			}
			fmt.Printf("row: %sv\n", result.Record().String())
		}
		if result.Err() != nil {
			t.Error(result.Err())
		}
	}

}

func TestAuthorizationsAPI(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", authToken)
	authAPI := client.AuthorizationsAPI()
	listRes, err := authAPI.GetAuthorizations(context.Background())
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 1)

	orgName := "my-org"
	org, err := client.OrganizationsAPI().FindOrganizationByName(context.Background(), orgName)
	require.Nil(t, err)
	require.NotNil(t, org)
	assert.Equal(t, orgName, org.Name)

	permission := &domain.Permission{
		Action: domain.PermissionActionWrite,
		Resource: domain.Resource{
			Type: domain.ResourceTypeBuckets,
		},
	}
	permissions := []domain.Permission{*permission}

	auth, err := authAPI.CreateAuthorizationWithOrgID(context.Background(), *org.Id, permissions)
	require.Nil(t, err)
	require.NotNil(t, auth)
	assert.Equal(t, domain.AuthorizationUpdateRequestStatusActive, *auth.Status, *auth.Status)

	listRes, err = authAPI.GetAuthorizations(context.Background())
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 2)

	listRes, err = authAPI.FindAuthorizationsByUserName(context.Background(), "my-user")
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 2)

	listRes, err = authAPI.FindAuthorizationsByOrgID(context.Background(), *org.Id)
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 2)

	listRes, err = authAPI.FindAuthorizationsByOrgName(context.Background(), "my-org")
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 2)

	listRes, err = authAPI.FindAuthorizationsByOrgName(context.Background(), "not-existent-org")
	require.Nil(t, listRes)
	require.NotNil(t, err)
	//assert.Len(t, *listRes, 0)

	auth, err = authAPI.UpdateAuthorizationStatus(context.Background(), auth, domain.AuthorizationUpdateRequestStatusInactive)
	require.Nil(t, err)
	require.NotNil(t, auth)
	assert.Equal(t, domain.AuthorizationUpdateRequestStatusInactive, *auth.Status, *auth.Status)

	listRes, err = authAPI.GetAuthorizations(context.Background())
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 2)

	err = authAPI.DeleteAuthorization(context.Background(), auth)
	require.Nil(t, err)

	listRes, err = authAPI.GetAuthorizations(context.Background())
	require.Nil(t, err)
	require.NotNil(t, listRes)
	assert.Len(t, *listRes, 1)

}

func TestOrganizationsAPI(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", authToken)
	orgsAPI := client.OrganizationsAPI()
	usersAPI := client.UsersAPI()
	orgName := "my-org-2"
	orgDescription := "my-org 2 description"
	ctx := context.Background()
	invalidID := "aaaaaaaaaaaaaaaa"

	orgList, err := orgsAPI.GetOrganizations(ctx)
	require.Nil(t, err)
	require.NotNil(t, orgList)
	assert.Len(t, *orgList, 1)

	//test error
	org, err := orgsAPI.CreateOrganizationWithName(ctx, "")
	assert.NotNil(t, err)
	require.Nil(t, org)

	org, err = orgsAPI.CreateOrganizationWithName(ctx, orgName)
	require.Nil(t, err)
	require.NotNil(t, org)
	assert.Equal(t, orgName, org.Name)

	//test duplicit org
	_, err = orgsAPI.CreateOrganizationWithName(ctx, orgName)
	require.NotNil(t, err)

	org.Description = &orgDescription

	org, err = orgsAPI.UpdateOrganization(ctx, org)
	require.Nil(t, err)
	require.NotNil(t, org)
	assert.Equal(t, orgDescription, *org.Description)

	orgList, err = orgsAPI.GetOrganizations(ctx)

	require.Nil(t, err)
	require.NotNil(t, orgList)
	assert.Len(t, *orgList, 2)

	permission := &domain.Permission{
		Action: domain.PermissionActionWrite,
		Resource: domain.Resource{
			Type: domain.ResourceTypeBuckets,
		},
	}
	permissions := []domain.Permission{*permission}

	//create authorization for new org
	auth, err := client.AuthorizationsAPI().CreateAuthorizationWithOrgID(context.Background(), *org.Id, permissions)
	require.Nil(t, err)
	require.NotNil(t, auth)

	// create client with new auth token without permission
	clientOrg2 := influxdb2.NewClient("http://localhost:9999", *auth.Token)

	orgList, err = clientOrg2.OrganizationsAPI().GetOrganizations(ctx)
	require.Nil(t, err)
	require.NotNil(t, orgList)
	assert.Len(t, *orgList, 0)

	org2, err := orgsAPI.FindOrganizationByName(ctx, orgName)
	require.Nil(t, err)
	require.NotNil(t, org2)

	//find unknown org
	org2, err = orgsAPI.FindOrganizationByName(ctx, "not-existetn-org")
	assert.NotNil(t, err)
	assert.Nil(t, org2)

	//find org using token without org permission
	org2, err = clientOrg2.OrganizationsAPI().FindOrganizationByName(ctx, org.Name)
	assert.NotNil(t, err)
	assert.Nil(t, org2)

	err = client.AuthorizationsAPI().DeleteAuthorization(ctx, auth)
	require.Nil(t, err)

	members, err := orgsAPI.GetMembers(ctx, org)
	require.Nil(t, err)
	require.NotNil(t, members)
	require.Len(t, *members, 0)

	user, err := usersAPI.CreateUserWithName(ctx, "user-01")
	require.Nil(t, err)
	require.NotNil(t, user)

	member, err := orgsAPI.AddMember(ctx, org, user)
	require.Nil(t, err)
	require.NotNil(t, member)
	assert.Equal(t, *user.Id, *member.Id)
	assert.Equal(t, user.Name, member.Name)

	// Add member with invalid id
	member, err = orgsAPI.AddMemberWithID(ctx, *org.Id, invalidID)
	assert.NotNil(t, err)
	assert.Nil(t, member)

	members, err = orgsAPI.GetMembers(ctx, org)
	require.Nil(t, err)
	require.NotNil(t, members)
	require.Len(t, *members, 1)

	// get member with invalid id
	members, err = orgsAPI.GetMembersWithID(ctx, invalidID)
	assert.NotNil(t, err)
	assert.Nil(t, members)

	org2, err = orgsAPI.FindOrganizationByID(ctx, *org.Id)
	require.Nil(t, err)
	require.NotNil(t, org2)
	assert.Equal(t, org.Name, org2.Name)

	// find invalid id
	org2, err = orgsAPI.FindOrganizationByID(ctx, invalidID)
	assert.NotNil(t, err)
	assert.Nil(t, org2)

	orgs, err := orgsAPI.FindOrganizationsByUserID(ctx, *user.Id)
	require.Nil(t, err)
	require.NotNil(t, orgs)
	require.Len(t, *orgs, 1)
	assert.Equal(t, org.Name, (*orgs)[0].Name)

	// look for not existent
	orgs, err = orgsAPI.FindOrganizationsByUserID(ctx, invalidID)
	assert.Nil(t, err)
	assert.NotNil(t, orgs)
	assert.Len(t, *orgs, 0)

	orgName2 := "my-org-3"

	org2, err = orgsAPI.CreateOrganizationWithName(ctx, orgName2)
	require.Nil(t, err)
	require.NotNil(t, org2)
	assert.Equal(t, orgName2, org2.Name)

	orgList, err = orgsAPI.GetOrganizations(ctx)
	require.Nil(t, err)
	require.NotNil(t, orgList)
	assert.Len(t, *orgList, 3)

	owners, err := orgsAPI.GetOwners(ctx, org2)
	assert.Nil(t, err)
	assert.NotNil(t, owners)
	assert.Len(t, *owners, 1)

	//get owners with invalid id
	owners, err = orgsAPI.GetOwnersWithID(ctx, invalidID)
	assert.NotNil(t, err)
	assert.Nil(t, owners)

	owner, err := orgsAPI.AddOwner(ctx, org2, user)
	require.Nil(t, err)
	require.NotNil(t, owner)

	// add owner with invalid ID
	owner, err = orgsAPI.AddOwnerWithID(ctx, *org2.Id, invalidID)
	assert.NotNil(t, err)
	assert.Nil(t, owner)

	owners, err = orgsAPI.GetOwners(ctx, org2)
	require.Nil(t, err)
	require.NotNil(t, owners)
	assert.Len(t, *owners, 2)

	u, err := usersAPI.FindUserByName(ctx, "my-user")
	require.Nil(t, err)
	require.NotNil(t, u)

	err = orgsAPI.RemoveOwner(ctx, org2, u)
	require.Nil(t, err)

	// remove owner with invalid ID
	err = orgsAPI.RemoveOwnerWithID(ctx, invalidID, invalidID)
	assert.NotNil(t, err)

	owners, err = orgsAPI.GetOwners(ctx, org2)
	require.Nil(t, err)
	require.NotNil(t, owners)
	assert.Len(t, *owners, 1)

	orgs, err = orgsAPI.FindOrganizationsByUserID(ctx, *user.Id)
	require.Nil(t, err)
	require.NotNil(t, orgs)
	require.Len(t, *orgs, 2)

	err = orgsAPI.RemoveMember(ctx, org, user)
	require.Nil(t, err)

	// remove invalid memberID
	err = orgsAPI.RemoveMemberWithID(ctx, invalidID, invalidID)
	assert.NotNil(t, err)

	members, err = orgsAPI.GetMembers(ctx, org)
	require.Nil(t, err)
	require.NotNil(t, members)
	require.Len(t, *members, 0)

	err = orgsAPI.DeleteOrganization(ctx, org)
	require.Nil(t, err)

	err = orgsAPI.DeleteOrganization(ctx, org2)
	assert.Nil(t, err)

	// delete invalid org
	err = orgsAPI.DeleteOrganizationWithID(ctx, invalidID)
	assert.NotNil(t, err)

	orgList, err = orgsAPI.GetOrganizations(ctx)
	require.Nil(t, err)
	require.NotNil(t, orgList)
	assert.Len(t, *orgList, 1)

	err = usersAPI.DeleteUser(ctx, user)
	require.Nil(t, err)

}

func TestUsersAPI(t *testing.T) {
	client := influxdb2.NewClient("http://localhost:9999", authToken)

	usersAPI := client.UsersAPI()

	me, err := usersAPI.Me(context.Background())
	require.Nil(t, err)
	require.NotNil(t, me)

	users, err := usersAPI.GetUsers(context.Background())
	require.Nil(t, err)
	require.NotNil(t, users)
	assert.Len(t, *users, 1)

	user, err := usersAPI.CreateUserWithName(context.Background(), "user-01")
	require.Nil(t, err)
	require.NotNil(t, user)

	users, err = usersAPI.GetUsers(context.Background())
	require.Nil(t, err)
	require.NotNil(t, users)
	assert.Len(t, *users, 2)

	status := domain.UserStatusInactive
	user.Status = &status
	user, err = usersAPI.UpdateUser(context.Background(), user)
	require.Nil(t, err)
	require.NotNil(t, user)
	assert.Equal(t, status, *user.Status)

	user, err = usersAPI.FindUserByID(context.Background(), *user.Id)
	require.Nil(t, err)
	require.NotNil(t, user)

	err = usersAPI.UpdateUserPassword(context.Background(), user, "my-password")
	require.Nil(t, err)

	err = usersAPI.DeleteUser(context.Background(), user)
	require.Nil(t, err)

	users, err = usersAPI.GetUsers(context.Background())
	require.Nil(t, err)
	require.NotNil(t, users)
	assert.Len(t, *users, 1)
}

func TestDeleteAPI(t *testing.T) {
	ctx := context.Background()
	client := influxdb2.NewClient("http://localhost:9999", authToken)
	writeAPI := client.WriteAPIBlocking("my-org", "my-bucket")
	queryAPI := client.QueryAPI("my-org")
	tmStart := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	writeF := func(start time.Time, count int64) time.Time {
		tm := start
		for i, f := int64(0), 0.0; i < count; i++ {
			p := write.NewPoint("test",
				map[string]string{"a": strconv.FormatInt(i%2, 10), "b": "static"},
				map[string]interface{}{"f": f, "i": i},
				tm)
			err := writeAPI.WritePoint(ctx, p)
			require.Nil(t, err, err)
			f += 1.2
			tm = tm.Add(time.Minute)
		}
		return tm
	}
	countF := func(start, stop time.Time) int64 {
		result, err := queryAPI.Query(ctx, `from(bucket:"my-bucket")|> range(start: `+start.Format(time.RFC3339)+`, stop:`+stop.Format(time.RFC3339)+`) 
		|> filter(fn: (r) => r._measurement == "test" and r._field == "f")
		|> drop(columns: ["a", "b"])
		|> pivot(rowKey:["_time"], columnKey: ["_field"], valueColumn: "_value")
		|> count(column: "f")`)

		require.Nil(t, err, err)
		count := int64(0)
		if result.Next() {
			require.NotNil(t, result.Record().ValueByKey("f"))
			count = result.Record().ValueByKey("f").(int64)
		}
		return count
	}
	tmEnd := writeF(tmStart, 100)
	assert.Equal(t, int64(100), countF(tmStart, tmEnd))
	deleteAPI := client.DeleteAPI()

	org, err := client.OrganizationsAPI().FindOrganizationByName(ctx, "my-org")
	require.Nil(t, err, err)
	require.NotNil(t, org)

	bucket, err := client.BucketsAPI().FindBucketByName(ctx, "my-bucket")
	require.Nil(t, err, err)
	require.NotNil(t, bucket)

	err = deleteAPI.DeleteWithName(ctx, "my-org", "my-bucket", tmStart, tmEnd, "")
	require.Nil(t, err, err)
	assert.Equal(t, int64(0), countF(tmStart, tmEnd))

	tmEnd = writeF(tmStart, 100)
	assert.Equal(t, int64(100), countF(tmStart, tmEnd))

	err = deleteAPI.DeleteWithID(ctx, *org.Id, *bucket.Id, tmStart, tmEnd, "a=1")
	require.Nil(t, err, err)
	assert.Equal(t, int64(50), countF(tmStart, tmEnd))

	err = deleteAPI.Delete(ctx, org, bucket, tmStart.Add(50*time.Minute), tmEnd, "b=static")
	require.Nil(t, err, err)
	assert.Equal(t, int64(25), countF(tmStart, tmEnd))

	err = deleteAPI.DeleteWithName(ctx, "org", "my-bucket", tmStart.Add(50*time.Minute), tmEnd, "b=static")
	require.NotNil(t, err, err)
	assert.True(t, strings.Contains(err.Error(), "not found"))

	err = deleteAPI.DeleteWithName(ctx, "my-org", "bucket", tmStart.Add(50*time.Minute), tmEnd, "b=static")
	require.NotNil(t, err, err)
	assert.True(t, strings.Contains(err.Error(), "not found"))
}

func TestBucketsAPI(t *testing.T) {
	ctx := context.Background()
	client := influxdb2.NewClient("http://localhost:9999", authToken)

	bucketsAPI := client.BucketsAPI()

	buckets, err := bucketsAPI.GetBuckets(ctx)
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	//at least three buckets, my-bucket and two system buckets.
	assert.True(t, len(*buckets) > 2)

	// test find existing bucket
	bucket, err := bucketsAPI.FindBucketByName(ctx, "my-bucket")
	require.Nil(t, err, err)
	require.NotNil(t, bucket)
	assert.Equal(t, "my-bucket", bucket.Name)

	//test find non-existing bucket
	bucket, err = bucketsAPI.FindBucketByName(ctx, "not existing bucket")
	assert.NotNil(t, err)
	assert.Nil(t, bucket)

	// create organizatiton for bucket
	org, err := client.OrganizationsAPI().CreateOrganizationWithName(ctx, "bucket-org")
	require.Nil(t, err)
	require.NotNil(t, org)

	// collect all buckets including system ones created for new organization
	buckets, err = bucketsAPI.GetBuckets(ctx)
	require.Nil(t, err, err)
	//store #all buckets before creating new ones
	bucketsNum := len(*buckets)

	name := "bucket-x"
	b, err := bucketsAPI.CreateBucketWithName(ctx, org, name, domain.RetentionRule{EverySeconds: 3600 * 1}, domain.RetentionRule{EverySeconds: 3600 * 24})
	require.Nil(t, err, err)
	require.NotNil(t, b)
	assert.Equal(t, name, b.Name)
	assert.Len(t, b.RetentionRules, 1)

	// Test update
	desc := "bucket description"
	b.Description = &desc
	b.RetentionRules = []domain.RetentionRule{{EverySeconds: 60}}
	b, err = bucketsAPI.UpdateBucket(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, b)
	assert.Equal(t, name, b.Name)
	assert.Equal(t, desc, *b.Description)
	assert.Len(t, b.RetentionRules, 1)

	// Test owners
	userOwner, err := client.UsersAPI().CreateUserWithName(ctx, "bucket-owner")
	require.Nil(t, err, err)
	require.NotNil(t, userOwner)

	owners, err := bucketsAPI.GetOwners(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, owners)
	assert.Len(t, *owners, 0)

	owner, err := bucketsAPI.AddOwner(ctx, b, userOwner)
	require.Nil(t, err, err)
	require.NotNil(t, owner)
	assert.Equal(t, *userOwner.Id, *owner.Id)

	owners, err = bucketsAPI.GetOwners(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, owners)
	assert.Len(t, *owners, 1)

	err = bucketsAPI.RemoveOwnerWithID(ctx, *b.Id, *(&(*owners)[0]).Id)
	require.Nil(t, err, err)

	owners, err = bucketsAPI.GetOwners(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, owners)
	assert.Len(t, *owners, 0)

	//test failures
	_, err = bucketsAPI.AddOwnerWithID(ctx, "000000000000000", *userOwner.Id)
	assert.NotNil(t, err)

	_, err = bucketsAPI.AddOwnerWithID(ctx, *b.Id, "000000000000000")
	assert.NotNil(t, err)

	_, err = bucketsAPI.GetOwnersWithID(ctx, "000000000000000")
	assert.NotNil(t, err)

	err = bucketsAPI.RemoveOwnerWithID(ctx, *b.Id, "000000000000000")
	assert.NotNil(t, err)

	err = bucketsAPI.RemoveOwnerWithID(ctx, "000000000000000", *userOwner.Id)
	assert.NotNil(t, err)

	// Test members
	userMember, err := client.UsersAPI().CreateUserWithName(ctx, "bucket-member")
	require.Nil(t, err, err)
	require.NotNil(t, userMember)

	members, err := bucketsAPI.GetMembers(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, members)
	assert.Len(t, *members, 0)

	member, err := bucketsAPI.AddMember(ctx, b, userMember)
	require.Nil(t, err, err)
	require.NotNil(t, member)
	assert.Equal(t, *userMember.Id, *member.Id)

	members, err = bucketsAPI.GetMembers(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, members)
	assert.Len(t, *members, 1)

	err = bucketsAPI.RemoveMemberWithID(ctx, *b.Id, *(&(*members)[0]).Id)
	require.Nil(t, err, err)

	members, err = bucketsAPI.GetMembers(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, members)
	assert.Len(t, *members, 0)

	//test failures
	_, err = bucketsAPI.AddMemberWithID(ctx, "000000000000000", *userMember.Id)
	assert.NotNil(t, err)

	_, err = bucketsAPI.AddMemberWithID(ctx, *b.Id, "000000000000000")
	assert.NotNil(t, err)

	_, err = bucketsAPI.GetMembersWithID(ctx, "000000000000000")
	assert.NotNil(t, err)

	err = bucketsAPI.RemoveMemberWithID(ctx, *b.Id, "000000000000000")
	assert.NotNil(t, err)

	err = bucketsAPI.RemoveMemberWithID(ctx, "000000000000000", *userMember.Id)
	assert.NotNil(t, err)

	err = bucketsAPI.DeleteBucketWithID(ctx, *b.Id)
	assert.Nil(t, err, err)

	err = client.UsersAPI().DeleteUser(ctx, userOwner)
	assert.Nil(t, err, err)

	err = client.UsersAPI().DeleteUser(ctx, userMember)
	assert.Nil(t, err, err)

	//test failures
	_, err = bucketsAPI.FindBucketByID(ctx, *b.Id)
	assert.NotNil(t, err)

	_, err = bucketsAPI.UpdateBucket(ctx, b)
	assert.NotNil(t, err)

	b.OrgID = b.Id
	_, err = bucketsAPI.CreateBucket(ctx, b)
	assert.NotNil(t, err)

	// create bucket by object
	b = &domain.Bucket{
		Description:    &desc,
		Name:           name,
		OrgID:          org.Id,
		RetentionRules: []domain.RetentionRule{{EverySeconds: 3600}},
	}

	b, err = bucketsAPI.CreateBucket(ctx, b)
	require.Nil(t, err, err)
	require.NotNil(t, b)
	assert.Equal(t, name, b.Name)
	assert.Equal(t, *org.Id, *b.OrgID)
	assert.Equal(t, desc, *b.Description)
	assert.Len(t, b.RetentionRules, 1)

	// fail duplicit name
	_, err = bucketsAPI.CreateBucketWithName(ctx, org, b.Name)
	assert.NotNil(t, err)

	// fail org not found
	_, err = bucketsAPI.CreateBucketWithNameWithID(ctx, *b.Id, b.Name)
	assert.NotNil(t, err)

	err = bucketsAPI.DeleteBucketWithID(ctx, *b.Id)
	assert.Nil(t, err, err)

	err = bucketsAPI.DeleteBucketWithID(ctx, *b.Id)
	assert.NotNil(t, err)

	// create new buckets inside org
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("bucket-%03d", i)
		b, err := bucketsAPI.CreateBucketWithName(ctx, org, name)
		require.Nil(t, err, err)
		require.NotNil(t, b)
		assert.Equal(t, name, b.Name)
	}

	// test paging, 1st page
	buckets, err = bucketsAPI.GetBuckets(ctx)
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	assert.Len(t, *buckets, 20)
	// test paging, 2nd, last page
	buckets, err = bucketsAPI.GetBuckets(ctx, api.PagingWithOffset(20))
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	//+2 is a bug, when using offset>4 there are returned also system buckets
	assert.Len(t, *buckets, 10+2+bucketsNum)
	// test paging with increase limit to cover all buckets
	buckets, err = bucketsAPI.GetBuckets(ctx, api.PagingWithLimit(100))
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	assert.Len(t, *buckets, 30+bucketsNum)
	// test filtering buckets by org id
	buckets, err = bucketsAPI.FindBucketsByOrgID(ctx, *org.Id, api.PagingWithLimit(100))
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	assert.Len(t, *buckets, 30+2)
	// test filtering buckets by org name
	buckets, err = bucketsAPI.FindBucketsByOrgName(ctx, org.Name, api.PagingWithLimit(100))
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	assert.Len(t, *buckets, 30+2)
	// delete buckete
	for _, b := range *buckets {
		if strings.HasPrefix(b.Name, "bucket-") {
			err = bucketsAPI.DeleteBucket(ctx, &b)
			assert.Nil(t, err, err)
		}
	}
	// check all created buckets deleted
	buckets, err = bucketsAPI.FindBucketsByOrgName(ctx, org.Name, api.PagingWithLimit(100))
	require.Nil(t, err, err)
	require.NotNil(t, buckets)
	assert.Len(t, *buckets, 2)

	err = client.OrganizationsAPI().DeleteOrganization(ctx, org)
	assert.Nil(t, err, err)

	// should fail with org not found
	_, err = bucketsAPI.FindBucketsByOrgName(ctx, org.Name, api.PagingWithLimit(100))
	assert.NotNil(t, err)
}

func TestLabelsAPI(t *testing.T) {
	client := influxdb2.NewClientWithOptions("http://localhost:9999", authToken, influxdb2.DefaultOptions().SetLogLevel(3))
	labelsAPI := client.LabelsAPI()
	orgAPI := client.OrganizationsAPI()

	ctx := context.Background()

	myorg, err := orgAPI.FindOrganizationByName(ctx, "my-org")
	require.Nil(t, err, err)
	require.NotNil(t, myorg)

	labels, err := labelsAPI.GetLabels(ctx)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 0)

	labelName := "Active State"
	props := map[string]string{"color": "#33ffddd", "description": "Marks org active"}
	label, err := labelsAPI.CreateLabelWithName(ctx, myorg, labelName, props)
	require.Nil(t, err, err)
	require.NotNil(t, label)
	assert.Equal(t, labelName, *label.Name)
	require.NotNil(t, label.Properties)
	assert.Equal(t, props, label.Properties.AdditionalProperties)

	//remove properties
	label.Properties.AdditionalProperties = map[string]string{"color": "", "description": ""}
	label2, err := labelsAPI.UpdateLabel(ctx, label)
	require.Nil(t, err, err)
	require.NotNil(t, label2)
	assert.Equal(t, labelName, *label2.Name)
	assert.Nil(t, label2.Properties)

	label2, err = labelsAPI.FindLabelByID(ctx, *label.Id)
	require.Nil(t, err, err)
	require.NotNil(t, label2)
	assert.Equal(t, labelName, *label2.Name)

	label2, err = labelsAPI.FindLabelByID(ctx, "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	label2, err = labelsAPI.FindLabelByName(ctx, *myorg.Id, labelName)
	require.Nil(t, err, err)
	require.NotNil(t, label2)
	assert.Equal(t, labelName, *label2.Name)

	label2, err = labelsAPI.FindLabelByName(ctx, *myorg.Id, "wrong label")
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	labels, err = labelsAPI.GetLabels(ctx)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 1)

	labels, err = labelsAPI.FindLabelsByOrg(ctx, myorg)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 1)

	labels, err = labelsAPI.FindLabelsByOrgID(ctx, *myorg.Id)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 1)

	labels, err = labelsAPI.FindLabelsByOrgID(ctx, "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, labels)

	// duplicate label
	label2, err = labelsAPI.CreateLabelWithName(ctx, myorg, labelName, nil)
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	labels, err = orgAPI.GetLabels(ctx, myorg)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 0)

	org, err := orgAPI.CreateOrganizationWithName(ctx, "org1")
	require.Nil(t, err, err)
	require.NotNil(t, org)

	labels, err = orgAPI.GetLabels(ctx, org)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 0)

	labelx, err := orgAPI.AddLabel(ctx, org, label)
	require.Nil(t, err, err)
	require.NotNil(t, labelx)

	labels, err = orgAPI.GetLabels(ctx, org)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 1)

	err = orgAPI.RemoveLabel(ctx, org, label)
	require.Nil(t, err, err)

	labels, err = orgAPI.GetLabels(ctx, org)
	require.Nil(t, err, err)
	require.NotNil(t, labels)
	assert.Len(t, *labels, 0)

	labels, err = orgAPI.GetLabelsWithID(ctx, "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, labels)

	label2, err = orgAPI.AddLabelWithID(ctx, *org.Id, "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	label2, err = orgAPI.AddLabelWithID(ctx, "000000000000000", "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	err = orgAPI.RemoveLabelWithID(ctx, *org.Id, "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	err = orgAPI.RemoveLabelWithID(ctx, "000000000000000", "000000000000000")
	require.NotNil(t, err, err)
	require.Nil(t, label2)

	err = orgAPI.DeleteOrganization(ctx, org)
	assert.Nil(t, err, err)

	err = labelsAPI.DeleteLabel(ctx, label)
	require.Nil(t, err, err)

	err = labelsAPI.DeleteLabel(ctx, label)
	require.NotNil(t, err, err)
}
