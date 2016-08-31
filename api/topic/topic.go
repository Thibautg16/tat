package topic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/ovh/tat"
	"github.com/ovh/tat/api/group"
	"github.com/ovh/tat/api/store"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// InitDB creates /Private topic if necessary
func InitDB() {
	nbTopics, err := CountTopics()
	if err != nil {
		log.Fatalf("Error with getting Mongodb.Instance %s", err)
		return
	}

	if nbTopics == 0 {
		// Create /Private topic
		InitPrivateTopic()
	}
}

func buildTopicCriteria(criteria *tat.TopicCriteria, user *tat.User) (bson.M, error) {
	var query = []bson.M{}

	if criteria.IDTopic != "" {
		queryIDTopics := bson.M{}
		queryIDTopics["$or"] = []bson.M{}
		for _, val := range strings.Split(criteria.IDTopic, ",") {
			queryIDTopics["$or"] = append(queryIDTopics["$or"].([]bson.M), bson.M{"_id": val})
		}
		query = append(query, queryIDTopics)
	}
	if criteria.Topic != "" || criteria.OnlyFavorites == tat.True {
		queryTopics := bson.M{}
		queryTopics["$or"] = []bson.M{}
		for _, val := range strings.Split(criteria.Topic, ",") {
			queryTopics["$or"] = append(queryTopics["$or"].([]bson.M), bson.M{"topic": val})
		}
		query = append(query, queryTopics)
	}
	if criteria.TopicPath != "" {
		query = append(query, bson.M{"topic": bson.RegEx{Pattern: "^" + regexp.QuoteMeta(criteria.TopicPath) + ".*$", Options: "im"}})
	}
	if criteria.Description != "" {
		queryDescriptions := bson.M{}
		queryDescriptions["$or"] = []bson.M{}
		for _, val := range strings.Split(criteria.Description, ",") {
			queryDescriptions["$or"] = append(queryDescriptions["$or"].([]bson.M), bson.M{"description": val})
		}
		query = append(query, queryDescriptions)
	}
	if criteria.Group != "" {
		queryGroups := bson.M{}
		queryGroups["$or"] = []bson.M{}
		queryGroups["$or"] = append(queryGroups["$or"].([]bson.M), bson.M{"adminGroups": bson.M{"$in": strings.Split(criteria.Group, ",")}})
		queryGroups["$or"] = append(queryGroups["$or"].([]bson.M), bson.M{"roGroups": bson.M{"$in": strings.Split(criteria.Group, ",")}})
		queryGroups["$or"] = append(queryGroups["$or"].([]bson.M), bson.M{"rwGroups": bson.M{"$in": strings.Split(criteria.Group, ",")}})
		query = append(query, queryGroups)
	}

	var bsonDate = bson.M{}

	if criteria.DateMinCreation != "" {
		i, err := strconv.ParseInt(criteria.DateMinCreation, 10, 64)
		if err != nil {
			return bson.M{}, fmt.Errorf("Error while parsing dateMinCreation %s", err)
		}
		tm := time.Unix(i, 0)
		bsonDate["$gte"] = tm.Unix()
	}
	if criteria.DateMaxCreation != "" {
		i, err := strconv.ParseInt(criteria.DateMaxCreation, 10, 64)
		if err != nil {
			return bson.M{}, fmt.Errorf("Error while parsing dateMaxCreation %s", err)
		}
		tm := time.Unix(i, 0)
		bsonDate["$lte"] = tm.Unix()
	}
	if len(bsonDate) > 0 {
		query = append(query, bson.M{"dateCreation": bsonDate})
	}

	if criteria.GetForAllTasksTopics {
		query = append(query, bson.M{
			"topic": bson.RegEx{Pattern: "^\\/Private\\/.*/Tasks", Options: "i"},
		})
	} else if criteria.GetForTatAdmin == tat.True && user.IsAdmin {
		// requester is tat Admin and wants all topics, except /Private/* topics
		query = append(query, bson.M{
			"topic": bson.M{"$not": bson.RegEx{Pattern: "^\\/Private\\/.*", Options: "i"}},
		})
	} else if criteria.GetForTatAdmin == tat.True && !user.IsAdmin {
		log.Warnf("User %s (not a TatAdmin) try to list all topics as an admin", user.Username)
	} else {
		bsonUser := []bson.M{}
		bsonUser = append(bsonUser, bson.M{"roUsers": bson.M{"$in": [1]string{user.Username}}})
		bsonUser = append(bsonUser, bson.M{"rwUsers": bson.M{"$in": [1]string{user.Username}}})
		bsonUser = append(bsonUser, bson.M{"adminUsers": bson.M{"$in": [1]string{user.Username}}})
		userGroups, err := group.GetUserGroupsOnlyName(user.Username)
		if err != nil {
			log.Errorf("Error with getting groups for user %s", err)
		} else {
			bsonUser = append(bsonUser, bson.M{"roGroups": bson.M{"$in": userGroups}})
			bsonUser = append(bsonUser, bson.M{"rwGroups": bson.M{"$in": userGroups}})
			bsonUser = append(bsonUser, bson.M{"adminGroups": bson.M{"$in": userGroups}})
		}
		query = append(query, bson.M{"$or": bsonUser})
	}

	if len(query) > 0 {
		return bson.M{"$and": query}, nil
	} else if len(query) == 1 {
		return query[0], nil
	}
	return bson.M{}, nil
}

// GetTopicSelectedFields return allowed selected field on mongo
func GetTopicSelectedFields(isAdmin, withTags, withLabels, oneTopic bool) bson.M {
	b := bson.M{}

	if isAdmin {
		b = bson.M{
			"_id":                  1,
			"collection":           1,
			"topic":                1,
			"description":          1,
			"roGroups":             1,
			"rwGroups":             1,
			"roUsers":              1,
			"rwUsers":              1,
			"adminUsers":           1,
			"adminGroups":          1,
			"maxlength":            1,
			"canForceDate":         1,
			"canUpdateMsg":         1,
			"canDeleteMsg":         1,
			"canUpdateAllMsg":      1,
			"canDeleteAllMsg":      1,
			"adminCanUpdateAllMsg": 1,
			"adminCanDeleteAllMsg": 1,
			"isAutoComputeTags":    1,
			"isAutoComputeLabels":  1,
			"isROPublic":           1,
			"dateModificationn":    1,
			"dateCreation":         1,
			"dateLastMessage":      1,
			"parameters":           1,
		}
		if oneTopic {
			b["history"] = 1
		}
	} else {
		b = bson.M{
			"collection":           1,
			"topic":                1,
			"description":          1,
			"isROPublic":           1,
			"canForceDate":         1,
			"canUpdateMsg":         1,
			"canDeleteMsg":         1,
			"canUpdateAllMsg":      1,
			"canDeleteAllMsg":      1,
			"adminCanUpdateAllMsg": 1,
			"adminCanDeleteAllMsg": 1,
			"maxlength":            1,
			"dateLastMessage":      1,
			"parameters":           1,
		}
	}
	if withTags {
		b["tags"] = 1
	}
	if withLabels {
		b["labels"] = 1
	}
	return b
}

// CountTopics returns the total number of topics in db
func CountTopics() (int, error) {
	return store.Tat().CTopics.Count()
}

// FindAllTopicsWithCollections returns the total number of topics in db
func FindAllTopicsWithCollections() ([]tat.Topic, error) {
	var topics []tat.Topic
	err := store.Tat().CTopics.Find(bson.M{"collection": bson.M{"$exists": true, "$ne": ""}}).
		Select(bson.M{"_id": 1, "collection": 1, "topic": 1}).
		All(&topics)
	return topics, err
}

// ListTopics returns list of topics, matching criterias
func ListTopics(criteria *tat.TopicCriteria, user *tat.User) (int, []tat.Topic, error) {
	var topics []tat.Topic

	cursor, errl := listTopicsCursor(criteria, user)
	if errl != nil {
		return -1, topics, errl
	}
	count, errc := cursor.Count()
	if errc != nil {
		return count, topics, fmt.Errorf("Error while count Topics %s", errc)
	}

	err := cursor.Select(GetTopicSelectedFields(user.IsAdmin, false, false, false)).
		Sort("topic").
		Skip(criteria.Skip).
		Limit(criteria.Limit).
		All(&topics)

	if err != nil {
		log.Errorf("Error while Find Topics %s", err)
		return count, topics, err
	}

	if user.IsAdmin {
		return count, topics, err
	}

	var topicsUser []tat.Topic
	// Get all topics where user is admin
	topicsMember, err := getTopicsForMemberUser(user, nil)
	if err != nil {
		return count, topics, err
	}

	for _, topic := range topics {
		added := false
		for _, topicMember := range topicsMember {
			if topic.ID == topicMember.ID {
				topic.AdminGroups = topicMember.AdminGroups
				topic.AdminUsers = topicMember.AdminUsers
				topic.ROUsers = topicMember.ROUsers
				topic.RWUsers = topicMember.RWUsers
				topic.RWGroups = topicMember.RWGroups
				topic.ROGroups = topicMember.ROGroups
				topicsUser = append(topicsUser, topic)
				added = true
				break
			}
		}
		if !added {
			topicsUser = append(topicsUser, topic)
		}
	}

	return count, topicsUser, err
}

// getTopicsForMemberUser where user is an admin or a member
func getTopicsForMemberUser(u *tat.User, topic *tat.Topic) ([]tat.Topic, error) {
	var topics []tat.Topic

	userGroups, err := group.GetUserGroupsOnlyName(u.Username)
	c := bson.M{}
	c["$or"] = []bson.M{}
	c["$or"] = append(c["$or"].([]bson.M), bson.M{"adminUsers": bson.M{"$in": [1]string{u.Username}}})
	c["$or"] = append(c["$or"].([]bson.M), bson.M{"roUsers": bson.M{"$in": [1]string{u.Username}}})
	c["$or"] = append(c["$or"].([]bson.M), bson.M{"rwUsers": bson.M{"$in": [1]string{u.Username}}})
	if len(userGroups) > 0 {
		c["$or"] = append(c["$or"].([]bson.M), bson.M{"adminGroups": bson.M{"$in": userGroups}})
		c["$or"] = append(c["$or"].([]bson.M), bson.M{"roGroups": bson.M{"$in": userGroups}})
		c["$or"] = append(c["$or"].([]bson.M), bson.M{"rwGroups": bson.M{"$in": userGroups}})
	}

	if topic != nil {
		c["$and"] = []bson.M{}
		c["$and"] = append(c["$and"].([]bson.M), bson.M{"topic": topic.Topic})
	}

	if err = store.Tat().CTopics.Find(c).All(&topics); err != nil {
		log.Errorf("Error while getting topics for member user: %s", err.Error())
	}

	return topics, err
}

func listTopicsCursor(criteria *tat.TopicCriteria, user *tat.User) (*mgo.Query, error) {
	c, err := buildTopicCriteria(criteria, user)
	if err != nil {
		return nil, err
	}
	return store.Tat().CTopics.Find(c), nil
}

// InitPrivateTopic insert topic "/Private"
func InitPrivateTopic() {
	topic := &tat.Topic{
		ID:                   bson.NewObjectId().Hex(),
		Topic:                "/Private",
		Description:          "Private Topics",
		DateCreation:         time.Now().Unix(),
		MaxLength:            tat.DefaultMessageMaxSize,
		CanForceDate:         false,
		CanUpdateMsg:         false,
		CanDeleteMsg:         false,
		CanUpdateAllMsg:      false,
		CanDeleteAllMsg:      false,
		AdminCanUpdateAllMsg: false,
		AdminCanDeleteAllMsg: false,
		IsROPublic:           false,
		IsAutoComputeTags:    true,
		IsAutoComputeLabels:  true,
	}
	err := store.Tat().CTopics.Insert(topic)
	log.Infof("Initialize /Private Topic")
	if err != nil {
		log.Fatalf("Error while initialize /Private Topic %s", err)
	}
}

// Insert creates a new topic. User is read write on topic
func Insert(topic *tat.Topic, user *tat.User) error {
	err := CheckAndFixName(topic)
	if err != nil {
		return err
	}

	isParentRootTopic, parentTopic, err := getParentTopic(topic)
	if !isParentRootTopic {
		if err != nil {
			return fmt.Errorf("Parent Topic not found %s", topic.Topic)
		}

		// If user create a Topic in /Private/username, no check or RW to create
		if !strings.HasPrefix(topic.Topic, "/Private/"+user.Username) {
			// check if user can create topic in /topic
			hasRW := IsUserAdmin(parentTopic, user)
			if !hasRW {
				return fmt.Errorf("No RW access to parent topic %s", parentTopic.Topic)
			}
		}
	} else if !user.IsAdmin { // no parent topic, check admin
		return fmt.Errorf("No write access to create parent topic %s", topic.Topic)
	}

	var existing = &tat.Topic{}
	if err = FindByTopic(existing, topic.Topic, true, false, false, nil); err == nil {
		return fmt.Errorf("Topic Already Exists : %s", topic.Topic)
	}

	topic.ID = bson.NewObjectId().Hex()
	topic.DateCreation = time.Now().Unix()
	topic.MaxLength = tat.DefaultMessageMaxSize // topic MaxLenth messages
	topic.CanForceDate = false
	topic.IsROPublic = false
	topic.IsAutoComputeLabels = true
	topic.IsAutoComputeTags = true
	topic.Collection = "messages" + topic.ID

	if !isParentRootTopic {
		topic.ROGroups = parentTopic.ROGroups
		topic.RWGroups = parentTopic.RWGroups
		topic.ROUsers = parentTopic.ROUsers
		topic.RWUsers = parentTopic.RWUsers
		topic.AdminUsers = parentTopic.AdminUsers
		topic.AdminGroups = parentTopic.AdminGroups
		topic.MaxLength = parentTopic.MaxLength
		topic.CanForceDate = parentTopic.CanForceDate
		// topic.CanUpdateMsg can be set by user.createTopics for new users
		// with CanUpdateMsg=true
		if !topic.CanUpdateMsg {
			topic.CanUpdateMsg = parentTopic.CanUpdateMsg
		}
		// topic.CanDeleteMsg can be set by user.createTopics for new users
		// with CanDeleteMsg=true
		if !topic.CanDeleteMsg {
			topic.CanDeleteMsg = parentTopic.CanDeleteMsg
		}
		topic.CanUpdateAllMsg = parentTopic.CanUpdateAllMsg
		topic.CanDeleteAllMsg = parentTopic.CanDeleteAllMsg
		topic.AdminCanUpdateAllMsg = parentTopic.AdminCanUpdateAllMsg
		topic.AdminCanDeleteAllMsg = parentTopic.AdminCanDeleteAllMsg
		topic.IsROPublic = parentTopic.IsROPublic
		topic.IsAutoComputeTags = parentTopic.IsAutoComputeTags
		topic.IsAutoComputeLabels = parentTopic.IsAutoComputeLabels
		topic.Parameters = parentTopic.Parameters
	}

	if err = store.Tat().CTopics.Insert(topic); err != nil {
		log.Errorf("Error while inserting new topic %s", err)
	}

	if errC := store.Tat().Session.DB(store.DatabaseName).C(topic.Collection).Create(&mgo.CollectionInfo{ForceIdIndex: true}); errC != nil {
		log.Errorf("Error while create new collection %s", topic.Collection)
	}

	store.EnsureIndexesMessages(topic.Collection)

	h := fmt.Sprintf("create a new topic :%s", topic.Topic)
	err = addToHistory(topic, bson.M{"_id": topic.ID}, user.Username, h)
	if err != nil {
		log.Errorf("Error while inserting history for new topic %s", err)
	}

	return AddRwUser(topic, user.Username, user.Username, false)
}

// Delete deletes a topic from database
func Delete(topic *tat.Topic, user *tat.User) error {

	if topic.Collection != "" {
		if err := store.Tat().Session.DB(store.DatabaseName).C(topic.Collection).DropCollection(); err != nil {
			return fmt.Errorf("Error while drop collection for topic %s err: %s", topic.Topic, err)
		}
	}

	if err := store.Tat().CTopics.Remove(bson.M{"_id": topic.ID}); err != nil {
		return fmt.Errorf("Error while remove topic from topics collection: %s", err)
	}

	return nil
}

// Truncate removes all messages in a topic
func Truncate(topic *tat.Topic) (int, error) {
	var changeInfo *mgo.ChangeInfo
	var err error
	if topic.Collection != "" && topic.Collection != store.CollectionDefaultMessages {
		changeInfo, err = store.GetCMessages(topic.Collection).RemoveAll(bson.M{})
	} else {
		changeInfo, err = store.GetCMessages(topic.Collection).RemoveAll(bson.M{"topic": topic.Topic})
		// TODO remove this after remove defaultMessagesCollection
		changeInfoOld, errOld := store.GetCMessages(topic.Collection).RemoveAll(bson.M{"topics": bson.M{"$in": [1]string{topic.Topic}}})
		if errOld != nil {
			log.Warnf("Error while removing message with topics attribute: %s", errOld)
		} else {
			log.Infof("Remove %d message with old way, select on topics attribute", changeInfoOld.Removed)
		}
	}

	if err != nil {
		return 0, err
	}
	return changeInfo.Removed, err
}

// TruncateTags clears "cached" tags in topic
func TruncateTags(topic *tat.Topic) error {
	return store.Tat().CTopics.Update(
		bson.M{"_id": topic.ID},
		bson.M{"$unset": bson.M{"tags": ""}})
}

// TruncateLabels clears "cached" labels on a topic
func TruncateLabels(topic *tat.Topic) error {
	return store.Tat().CTopics.Update(
		bson.M{"_id": topic.ID},
		bson.M{"$unset": bson.M{"labels": ""}})
}

var topicsLastMsgUpdate map[string]int64
var syncLastMsgUpdate sync.Mutex

func init() {
	topicsLastMsgUpdate = make(map[string]int64)
	go updateLastMessageTopics()
}

func updateLastMessageTopics() {
	for {
		syncLastMsgUpdate.Lock()
		if len(topicsLastMsgUpdate) > 0 {
			workOnTopicsLastMsgUpdate()
		}
		syncLastMsgUpdate.Unlock()
		time.Sleep(10 * time.Second)
	}
}

func workOnTopicsLastMsgUpdate() {
	for topic, dateUpdate := range topicsLastMsgUpdate {
		err := store.Tat().CTopics.Update(
			bson.M{"topic": topic},
			bson.M{"$set": bson.M{"dateLastMessage": dateUpdate}})
		if err != nil {
			log.Errorf("Error while update last date message on topic %s, err:%s", topic, err)
		}
	}
	topicsLastMsgUpdate = make(map[string]int64)
}

// UpdateTopicLastMessage updates tags on topic
func UpdateTopicLastMessage(topic *tat.Topic, dateUpdateLastMsg time.Time) {
	syncLastMsgUpdate.Lock()
	topicsLastMsgUpdate[topic.Topic] = dateUpdateLastMsg.Unix()
	syncLastMsgUpdate.Unlock()
}

// UpdateTopicTags updates tags on topic
func UpdateTopicTags(topic *tat.Topic, tags []string) {
	if !topic.IsAutoComputeTags || len(tags) == 0 {
		return
	}

	update := false
	newTags := topic.Tags
	for _, tag := range tags {
		if !tat.ArrayContains(topic.Tags, tag) {
			update = true
			newTags = append(newTags, tag)
		}
	}

	if update {
		err := store.Tat().CTopics.Update(
			bson.M{"_id": topic.ID},
			bson.M{"$set": bson.M{"tags": newTags}})

		if err != nil {
			log.Errorf("UpdateTopicTags> Error while updating tags on topic")
		} else {
			log.Debugf("UpdateTopicTags> Topic %s ", topic.Topic)
		}
	}
}

// UpdateTopicLabels updates labels on topic
func UpdateTopicLabels(topic *tat.Topic, labels []tat.Label) {
	if !topic.IsAutoComputeLabels || len(labels) == 0 {
		return
	}

	update := false
	newLabels := topic.Labels
	for _, label := range labels {
		find := false
		for _, tlabel := range topic.Labels {
			if label.Text == tlabel.Text {
				find = true
				continue
			}
		}
		if !find {
			newLabels = append(newLabels, label)
			update = true
		}
	}

	if update {
		err := store.Tat().CTopics.Update(
			bson.M{"_id": topic.ID},
			bson.M{"$set": bson.M{"labels": newLabels}})

		if err != nil {
			log.Errorf("UpdateTopicLabels> Error while updating labels on topic")
		} else {
			log.Debugf("UpdateTopicLabels> Topic %s ", topic.Topic)
		}
	}
}

// ListTags returns all tags on one topic
func ListTags(topic tat.Topic) ([]string, error) {
	var tags []string
	err := store.GetCMessages(topic.Collection).
		Find(bson.M{"topic": topic.Topic}).
		Distinct("tags", &tags)
	if err != nil {
		log.Errorf("Error while getting tags on topic %s, err:%s", topic.Topic, err.Error())
	}
	return tags, err
}

// ComputeTags computes "cached" tags in topic
// initialize tags, one entry per tag (unique)
func ComputeTags(topic *tat.Topic) (int, error) {
	tags, err := ListTags(*topic)
	if err != nil {
		return 0, err
	}

	err = store.Tat().CTopics.Update(
		bson.M{"_id": topic.ID},
		bson.M{"$set": bson.M{"tags": tags}})

	return len(tags), err
}

// ListLabels returns all labels on one topic
func ListLabels(topic tat.Topic) ([]tat.Label, error) {
	var labels []tat.Label
	err := store.GetCMessages(topic.Collection).
		Find(bson.M{"topic": topic.Topic}).
		Distinct("labels", &labels)
	if err != nil {
		log.Errorf("Error while getting labels on topic %s, err:%s", topic.Topic, err.Error())
	}
	return labels, err
}

// ComputeLabels computes "cached" labels on a topic
// initialize labels, one entry per label (unicity with text & color)
func ComputeLabels(topic *tat.Topic) (int, error) {
	labels, err := ListLabels(*topic)
	if err != nil {
		return 0, err
	}

	err = store.Tat().CTopics.Update(
		bson.M{"_id": topic.ID},
		bson.M{"$set": bson.M{"labels": labels}})

	return len(labels), err
}

// AllTopicsComputeLabels computes Labels on all topics
func AllTopicsComputeLabels() (string, error) {
	var topics []tat.Topic
	err := store.Tat().CTopics.Find(bson.M{}).
		Select(GetTopicSelectedFields(true, false, false, false)).
		All(&topics)

	if err != nil {
		log.Errorf("Error while getting all topics for compute labels")
		return "", err
	}

	errTxt := ""
	infoTxt := ""
	for _, topic := range topics {
		if topic.IsAutoComputeLabels {
			n, err := ComputeLabels(&topic)
			if err != nil {
				log.Errorf("Error while compute labels on topic %s: %s", topic.Topic, err.Error())
				errTxt += fmt.Sprintf(" Error compute labels on topic %s", topic.Topic)
			} else {
				infoTxt += fmt.Sprintf(" %d labels computed on topic %s", n, topic.Topic)
				log.Infof(infoTxt)
			}
		}
	}

	if errTxt != "" {
		return infoTxt, fmt.Errorf(errTxt)
	}
	return infoTxt, nil
}

// AllTopicsComputeTags computes Tags on all topics
func AllTopicsComputeTags() (string, error) {
	var topics []tat.Topic
	err := store.Tat().CTopics.Find(bson.M{}).
		Select(GetTopicSelectedFields(true, false, false, false)).
		All(&topics)

	if err != nil {
		log.Errorf("Error while getting all topics for compute tags")
		return "", err
	}

	errTxt := ""
	infoTxt := ""
	for _, topic := range topics {
		if topic.IsAutoComputeTags {
			n, err := ComputeTags(&topic)
			if err != nil {
				log.Errorf("Error while compute tags on topic %s: %s", topic.Topic, err.Error())
				errTxt += fmt.Sprintf(" Error compute tags on topic %s", topic.Topic)
			} else {
				infoTxt += fmt.Sprintf(" %d tags computed on topic %s", n, topic.Topic)
				log.Infof(infoTxt)
			}
		}
	}

	if errTxt != "" {
		return infoTxt, fmt.Errorf(errTxt)
	}
	return infoTxt, nil
}

// AllTopicsSetParam computes Tags on all topics
func AllTopicsSetParam(key, value string) (string, error) {
	var topics []tat.Topic
	err := store.Tat().CTopics.Find(bson.M{}).
		Select(GetTopicSelectedFields(true, false, false, false)).
		All(&topics)

	if err != nil {
		log.Errorf("Error while getting all topics for set a param")
		return "", err
	}

	errTxt := ""
	nOk := 1
	for _, topic := range topics {
		if err := setAParam(&topic, key, value); err != nil {
			log.Errorf("Error while set param %s on topic %s: %s", key, topic.Topic, err.Error())
			errTxt += fmt.Sprintf(" Error set param %s on topic %s", key, topic.Topic)
		} else {
			log.Infof(" %s param setted on topic %s", key, topic.Topic)
			nOk++
		}
	}

	if errTxt != "" {
		return "", fmt.Errorf(errTxt)
	}

	return fmt.Sprintf("Param setted on %d topics", nOk), nil
}

// setAParam sets a param on one topic. Limited only of some attributes
func setAParam(topic *tat.Topic, key, value string) error {
	if key == "isAutoComputeTags" || key == "isAutoComputeLabels" {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("Error while set param %s whith value %s", key, value)
		}
		return setABoolParam(topic, key, v)
	}
	return fmt.Errorf("set param %s is an invalid action", key)
}

func setABoolParam(topic *tat.Topic, key string, value bool) error {
	if key != "isAutoComputeTags" && key != "isAutoComputeLabels" {
		return fmt.Errorf("set param %s is an invalid action", key)
	}

	err := store.Tat().CTopics.Update(
		bson.M{"_id": topic.ID},
		bson.M{"$set": bson.M{key: value}},
	)
	if err != nil {
		log.Errorf("Error while update topic %s, param %s with new value %t", topic.Topic, key, value)
	}
	return nil
}

// Get parent topic
// If it is a "root topic", like /myTopic, return true, nil, nil
func getParentTopic(topic *tat.Topic) (bool, *tat.Topic, error) {
	index := strings.LastIndex(topic.Topic, "/")
	if index == 0 {
		return true, nil, nil
	}
	var nameParent = topic.Topic[0:index]
	var parentTopic = &tat.Topic{}
	err := FindByTopic(parentTopic, nameParent, true, false, false, nil)
	if err != nil {
		log.Errorf("Error while fetching parent topic %s", err)
	}
	return false, parentTopic, err
}

// FindByTopic returns topic by topicName.
func FindByTopic(topic *tat.Topic, topicIn string, isAdmin, withTags, withLabels bool, user *tat.User) error {
	topic.Topic = topicIn
	if err := CheckAndFixName(topic); err != nil {
		return err
	}
	err := store.Tat().CTopics.Find(bson.M{"topic": topic.Topic}).
		Select(GetTopicSelectedFields(isAdmin, withTags, withLabels, true)).
		One(&topic)

	if err != nil || topic.ID == "" {
		username := ""
		if user != nil {
			username = user.Username
		}
		e := fmt.Sprintf("FindByTopic> Error while fetching topic %s, isAdmin:%t, username:%s", topic.Topic, isAdmin, username)
		log.Debugf(e)
		// TODO DM
		return fmt.Errorf(e)
	}

	if user != nil {
		// Get all topics where user is admin
		topicsMember, errTopicsMember := getTopicsForMemberUser(user, topic)
		if errTopicsMember != nil {
			return errTopicsMember
		}

		if len(topicsMember) == 1 {
			topic.AdminGroups = topicsMember[0].AdminGroups
			topic.AdminUsers = topicsMember[0].AdminUsers
			topic.ROUsers = topicsMember[0].ROUsers
			topic.RWUsers = topicsMember[0].RWUsers
			topic.RWGroups = topicsMember[0].RWGroups
			topic.ROGroups = topicsMember[0].ROGroups
		}
	}
	return err
}

// IsTopicExists return true if topic exists, false otherwise
func IsTopicExists(topicName string) bool {
	var t = tat.Topic{}
	return FindByTopic(&t, topicName, false, false, false, nil) == nil // no error, return true
}

// FindByID return topic, matching given id
func FindByID(topic *tat.Topic, id string, isAdmin bool, username string) error {
	err := store.Tat().CTopics.Find(bson.M{"_id": id}).
		Select(GetTopicSelectedFields(isAdmin, false, false, true)).
		One(&topic)
	if err != nil {
		log.Errorf("Error while fetching topic with id:%s isAdmin:%t username:%s", id, isAdmin, username)
	}
	return err
}

// SetParam update param maxLength, canForceDate, canUpdateMsg, canDeleteMsg,
// canUpdateAllMsg, canDeleteAllMsg, adminCanUpdateAllMsg, adminCanDeleteAllMsg, isROPublic, parameters on topic
func SetParam(topic *tat.Topic, username string, recursive bool, maxLength int,
	canForceDate, canUpdateMsg, canDeleteMsg, canUpdateAllMsg, canDeleteAllMsg, adminCanUpdateAllMsg, adminCanDeleteAllMsg,
	isROPublic, isAutoComputeTags, isAutoComputeLabels bool, parameters []tat.TopicParameter) error {

	var selector bson.M

	if recursive {
		selector = bson.M{"topic": bson.RegEx{Pattern: "^" + topic.Topic + ".*$"}}
	} else {
		selector = bson.M{"_id": topic.ID}
	}

	if maxLength <= 0 {
		maxLength = tat.DefaultMessageMaxSize
	}

	update := bson.M{
		"maxlength":            maxLength,
		"canForceDate":         canForceDate,
		"canUpdateMsg":         canUpdateMsg,
		"canDeleteMsg":         canDeleteMsg,
		"canUpdateAllMsg":      canUpdateAllMsg,
		"canDeleteAllMsg":      canDeleteAllMsg,
		"adminCanUpdateAllMsg": adminCanUpdateAllMsg,
		"adminCanDeleteAllMsg": adminCanDeleteAllMsg,
		"isROPublic":           isROPublic,
		"isAutoComputeTags":    isAutoComputeTags,
		"isAutoComputeLabels":  isAutoComputeLabels,
	}

	if parameters != nil {
		update["parameters"] = parameters
	}
	_, err := store.Tat().CTopics.UpdateAll(selector, bson.M{"$set": update})

	if err != nil {
		log.Errorf("Error while updateAll parameters : %s", err.Error())
		return err
	}
	h := fmt.Sprintf("update param to maxlength:%d, canForceDate:%t, canUpdateMsg:%t, canDeleteMsg:%t, canUpdateAllMsg:%t, canDeleteAllMsg:%t, adminCanDeleteAllMsg:%t isROPublic:%t, isAutoComputeTags:%t, isAutoComputeLabels:%t", maxLength, canForceDate, canUpdateMsg, canDeleteMsg, canUpdateAllMsg, canDeleteAllMsg, adminCanDeleteAllMsg, isROPublic, isAutoComputeTags, isAutoComputeLabels)
	return addToHistory(topic, selector, username, h)
}

func actionOnSetParameter(topic *tat.Topic, operand, set, admin string, newParam tat.TopicParameter, recursive bool, history string) error {

	var selector bson.M

	if recursive {
		selector = bson.M{"topic": bson.RegEx{Pattern: "^" + topic.Topic + ".*$"}}
	} else {
		selector = bson.M{"_id": topic.ID}
	}

	var err error
	if operand == "$pull" {
		_, err = store.Tat().CTopics.UpdateAll(
			selector,
			bson.M{operand: bson.M{set: bson.M{"key": newParam.Key}}},
		)
	} else {
		_, err = store.Tat().CTopics.UpdateAll(
			selector,
			bson.M{operand: bson.M{set: bson.M{"key": newParam.Key, "value": newParam.Value}}},
		)
	}

	if err != nil {
		return err
	}
	return addToHistory(topic, selector, admin, history+" "+newParam.Key+":"+newParam.Value)
}

func actionOnSet(topic *tat.Topic, operand, set, username, admin string, recursive bool, history string) error {

	var selector bson.M

	if recursive {
		selector = bson.M{"topic": bson.RegEx{Pattern: "^" + topic.Topic + ".*$"}}
	} else {
		selector = bson.M{"_id": topic.ID}
	}

	_, err := store.Tat().CTopics.UpdateAll(
		selector,
		bson.M{operand: bson.M{set: username}},
	)

	if err != nil {
		return err
	}
	return addToHistory(topic, selector, admin, history+" "+username)
}

// AddRoUser add a read only user to topic
func AddRoUser(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$addToSet", "roUsers", username, admin, recursive, "add to ro")
}

// AddRwUser add a read write user to topic
func AddRwUser(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$addToSet", "rwUsers", username, admin, recursive, "add to ro")
}

// AddAdminUser add a read write user to topic
func AddAdminUser(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$addToSet", "adminUsers", username, admin, recursive, "add to admin")
}

// RemoveRoUser removes a read only user from topic
func RemoveRoUser(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$pull", "roUsers", username, admin, recursive, "remove from ro")
}

// RemoveAdminUser removes a read only user from topic
func RemoveAdminUser(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$pull", "adminUsers", username, admin, recursive, "remove from admin")
}

// RemoveRwUser removes a read write user from topic
func RemoveRwUser(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$pull", "rwUsers", username, admin, recursive, "remove from rw")
}

// AddRoGroup add a read only group to topic
func AddRoGroup(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$addToSet", "roGroups", username, admin, recursive, "add to ro")
}

// AddRwGroup add a read write group to topic
func AddRwGroup(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$addToSet", "rwGroups", username, admin, recursive, "add to ro")
}

// AddAdminGroup add a admin group to topic
func AddAdminGroup(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$addToSet", "adminGroups", username, admin, recursive, "add to admin")
}

// RemoveAdminGroup removes a read write group from topic
func RemoveAdminGroup(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$pull", "adminGroups", username, admin, recursive, "remove from admin")
}

// RemoveRoGroup removes a read only group from topic
func RemoveRoGroup(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$pull", "roGroups", username, admin, recursive, "remove from ro")
}

// RemoveRwGroup removes a read write group from topic
func RemoveRwGroup(topic *tat.Topic, admin string, username string, recursive bool) error {
	return actionOnSet(topic, "$pull", "rwGroups", username, admin, recursive, "remove from rw")
}

// AddParameter add a parameter to the topic
func AddParameter(topic *tat.Topic, admin string, parameterKey string, parameterValue string, recursive bool) error {
	return actionOnSetParameter(topic, "$addToSet", "parameters", admin, tat.TopicParameter{Key: parameterKey, Value: parameterValue}, recursive, "add to parameter")
}

// RemoveParameter removes a read only user from topic
func RemoveParameter(topic *tat.Topic, admin string, parameterKey string, parameterValue string, recursive bool) error {
	return actionOnSetParameter(topic, "$pull", "parameters", admin, tat.TopicParameter{Key: parameterKey, Value: ""}, recursive, "remove from parameters")
}

func addToHistory(topic *tat.Topic, selector bson.M, user string, historyToAdd string) error {
	toAdd := strconv.FormatInt(time.Now().Unix(), 10) + " " + user + " " + historyToAdd
	_, err := store.Tat().CTopics.UpdateAll(
		selector,
		bson.M{"$addToSet": bson.M{"history": toAdd}},
	)
	return err
}

// GetUserRights return isRW, isAdmin for user
// Check personal access to topic, and group access
func GetUserRights(topic *tat.Topic, user *tat.User) (bool, bool) {

	isUserAdmin := tat.ArrayContains(topic.AdminUsers, user.Username)
	if isUserAdmin {
		return true, true
	}

	userGroups, err := group.GetGroups(user.Username)
	if err != nil {
		log.Errorf("Error while fetching user groups")
		return false, false
	}

	var groups []string
	for _, g := range userGroups {
		groups = append(groups, g.Name)
	}

	isUserRW := tat.ArrayContains(topic.RWUsers, user.Username)
	isRW := isUserRW || tat.ItemInBothArrays(topic.RWGroups, groups)
	isAdmin := isUserAdmin || tat.ItemInBothArrays(topic.AdminUsers, groups)
	return isRW, isAdmin
}

// IsUserReadAccess  return true if user has read access to topic
func IsUserReadAccess(topic *tat.Topic, user tat.User) bool {
	currentTopic := topic

	if topic.IsROPublic {
		return true
	}

	// if user not admin, reload topic with admin rights
	if !user.IsAdmin {
		currentTopic = &tat.Topic{}
		if e := FindByID(currentTopic, topic.ID, true, user.Username); e != nil {
			return false
		}
	}

	if tat.ArrayContains(currentTopic.ROUsers, user.Username) ||
		tat.ArrayContains(currentTopic.RWUsers, user.Username) ||
		tat.ArrayContains(currentTopic.AdminUsers, user.Username) {
		return true
	}
	userGroups, err := group.GetGroups(user.Username)
	if err != nil {
		log.Errorf("Error while fetching user groups for user %s", user.Username)
		return false
	}

	var groups []string
	for _, g := range userGroups {
		groups = append(groups, g.Name)
	}

	return tat.ItemInBothArrays(currentTopic.RWGroups, groups) ||
		tat.ItemInBothArrays(currentTopic.ROGroups, groups) ||
		tat.ItemInBothArrays(currentTopic.AdminGroups, groups)
}

// IsUserAdmin return true if user is Tat admin or is admin on this topic
// Check personal access to topic, and group access
func IsUserAdmin(topic *tat.Topic, user *tat.User) bool {

	if user.IsAdmin {
		return true
	}

	if tat.ArrayContains(topic.AdminUsers, user.Username) {
		return true
	}

	userGroups, err := group.GetGroups(user.Username)
	if err != nil {
		log.Errorf("Error while fetching user groups")
		return false
	}

	var groups []string
	for _, g := range userGroups {
		groups = append(groups, g.Name)
	}

	if tat.ItemInBothArrays(topic.AdminGroups, groups) {
		return true
	}

	// user is "Admin" on his /Private/usrname topics
	return strings.HasPrefix(topic.Topic, "/Private/"+user.Username)
}

// CheckAndFixName Add a / to topic name is it is not present
// return an error if length of name is < 4 or > 100
func CheckAndFixName(topic *tat.Topic) error {
	name, err := tat.CheckAndFixNameTopic(topic.Topic)
	if err != nil {
		return err
	}
	topic.Topic = name
	return nil
}

// ChangeUsernameOnTopics changes a username on topics, ro, rw, admin users
func ChangeUsernameOnTopics(oldUsername, newUsername string) {
	changeNameOnSet("username", "roUsers", oldUsername, newUsername)
	changeNameOnSet("username", "rwUsers", oldUsername, newUsername)
	changeNameOnSet("username", "adminUsers", oldUsername, newUsername)
	changeUsernameOnPrivateTopics(oldUsername, newUsername)
}

// ChangeGroupnameOnTopics updates group name on topics
func ChangeGroupnameOnTopics(oldGroupname, newGroupname string) error {
	if err := changeNameOnSet("groupname", "roGroups", oldGroupname, newGroupname); err != nil {
		return err
	}
	if err := changeNameOnSet("groupname", "rwGroups", oldGroupname, newGroupname); err != nil {
		return err
	}
	if err := changeNameOnSet("groupname", "adminGroups", oldGroupname, newGroupname); err != nil {
		return err
	}
	return nil
}

func changeNameOnSet(typeChange, set, oldname, newname string) error {
	_, err := store.Tat().CTopics.UpdateAll(
		bson.M{set: oldname},
		bson.M{"$set": bson.M{set + ".$": newname}})

	if err != nil {
		log.Errorf("Error while changes %s from %s to %s on Topics (%s) %s", typeChange, oldname, newname, set, err)
		return fmt.Errorf("Error while changes %s from %s to %s on Topics (%s)", typeChange, oldname, newname, set)
	}
	return nil
}

func changeUsernameOnPrivateTopics(oldUsername, newUsername string) error {
	var topics []tat.Topic

	err := store.Tat().CTopics.Find(
		bson.M{
			"topic": bson.RegEx{
				Pattern: "^/Private/" + oldUsername + ".*$", Options: "i",
			}}).All(&topics)

	if err != nil {
		log.Errorf("Error while getting topic with username %s for rename to %s on Topics %s", oldUsername, newUsername, err)
	}

	for _, topic := range topics {
		newTopicName := strings.Replace(topic.Topic, oldUsername, newUsername, 1)
		errUpdate := store.Tat().CTopics.Update(
			bson.M{"_id": topic.ID},
			bson.M{"$set": bson.M{"topic": newTopicName}},
		)
		if errUpdate != nil {
			log.Errorf("Error while update Topic name from %s to %s :%s", topic.Topic, newTopicName, errUpdate)
		}
	}

	return err
}

// MigrateToDedicatedTopic sets collection  attribute on topic
func MigrateToDedicatedTopic(topic *tat.Topic) error {

	if topic.Collection != "" {
		return fmt.Errorf("MigrateToDedicatedTopic> This topic is already dedicated on a collection")
	}

	topic.Collection = "messages" + topic.ID

	errUpdate := store.Tat().CTopics.Update(
		bson.M{"_id": topic.ID},
		bson.M{"$set": bson.M{"collection": topic.Collection}},
	)
	if errUpdate != nil {
		return fmt.Errorf("MigrateToDedicatedTopic> Error while update Topic collection:%s", errUpdate)
	}

	store.EnsureIndexesMessages(topic.Collection)

	return nil
}