package factory

import (
	"log"
	"strings"

	"github.com/komari-monitor/komari/utils/item"
)

var (
	senders                = make(map[string]IMessageSender)
	senderConstructor      = make(map[string]MessageSenderConstructor)
	senderConstructorLower = make(map[string]string) // lowercase name -> canonical name
	sendersAdditionalItems = make(map[string][]item.Item)
)

func RegisterMessageSender(constructor MessageSenderConstructor) {
	sender := constructor()
	if sender == nil {
		panic("Message sender constructor returned nil")
	}
	name := sender.GetName()
	senderConstructor[name] = constructor
	senderConstructorLower[strings.ToLower(name)] = name
	if _, exists := senders[name]; exists {
		log.Println("Message sender already registered: " + name)
	}
	senders[name] = sender

	// 使用反射来提取提供程序的配置字段
	config := sender.GetConfiguration()
	items := item.Parse(config)

	sendersAdditionalItems[name] = items
}

func GetConstructor(name string) (MessageSenderConstructor, bool) {
	constructor, exists := senderConstructor[name]
	if exists {
		return constructor, true
	}
	// 大小写不敏感 fallback：用小写名查找规范名
	if canonical, ok := senderConstructorLower[strings.ToLower(name)]; ok {
		log.Printf("Provider name '%s' matched '%s' (case-insensitive)", name, canonical)
		return senderConstructor[canonical], true
	}
	return nil, false
}

func GetSenderConfigs() map[string][]item.Item {
	return sendersAdditionalItems
}

func GetAllMessageSenders() map[string]IMessageSender {
	return senders
}

func GetAllMessageSenderNames() []string {
	names := make([]string, 0, len(senders))
	for name := range senders {
		names = append(names, name)
	}
	return names
}

func Initialize() {
	for _, sender := range senders {
		if err := sender.Init(); err != nil {
			log.Printf("Failed to initialize message sender %s: %v", sender.GetName(), err)
		}
	}
}
