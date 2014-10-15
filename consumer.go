package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"github.com/streadway/amqp"
)

type Consumer struct {
	ConsumerConfigs  []*ConsumerConfig      `yaml:"consumers"`
	apps             []*ConsumerApplication
}

func NewConsumer() *Consumer {
	consumer := new(Consumer)
	consumer.apps = make([]*ConsumerApplication, 0)
	return consumer
}

func (this *Consumer) OnRegister(event *RegisterEvent) {
	event.Group.Done()
}

func (this *Consumer) OnInit(event *InitEvent) {
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		for _, consumerConfig := range this.ConsumerConfigs {
			app := NewConsumerApplication()
			this.apps = append(this.apps, app)
			go app.Run(consumerConfig)
		}
	} else {
		FailExit("%v", err)
	}
}

func (this *Consumer) OnFinish(event *FinishEvent) {
	for _, app := range this.apps {
		app.Close()
	}
	event.Group.Done()
}

type ConsumerConfig struct {
	URI string          `yaml:"uri"`
	Bindings []*Binding `yaml:"bindings"`
}

type Binding struct {
	Name     string       `yaml:"name"`
	Exchange string       `yaml:"exchange"`
	Queue    string       `yaml:"queue"`
	Type     ExchangeType `yaml:"type"`
	Routing  string       `yaml:"routing"`
	Handlers int          `yaml:"handlers"`
}

type ExchangeType string

const (
	EXCHANGE_TYPE_DIRECT ExchangeType = "direct"
	EXCHANGE_TYPE_FANOUT              = "fanout"
	EXCHANGE_TYPE_TOPIC               = "topic"
)

type ConsumerApplication struct {
	connect *amqp.Connection
	channel *amqp.Channel
}

func NewConsumerApplication() *ConsumerApplication {
	return new(ConsumerApplication)
}

func (this *ConsumerApplication) Run(consumerConfig *ConsumerConfig) {
	connect, err := amqp.Dial(consumerConfig.URI)
	if err == nil {
		Info("got connection to %s, getting channel", consumerConfig.URI)
		this.connect = connect
		channel, err := connect.Channel()
		if err == nil {
			Info("got channel for %s", consumerConfig.URI)
			this.channel = channel
			for _, binding := range consumerConfig.Bindings {
				if len(binding.Type) == 0 {
					binding.Type = EXCHANGE_TYPE_DIRECT
				}
				if len(binding.Name) > 0 {
					binding.Exchange = binding.Name
					binding.Queue = binding.Name
				}

				Info("declaring exchange - %s", binding.Exchange)
				if err = channel.ExchangeDeclare(
					binding.Exchange,      // name of the exchange
					string(binding.Type),  // type
					true,                  // durable
					false,                 // delete when complete
					false,                 // internal
					false,                 // noWait
					nil,                   // arguments
				)
					err == nil {
					Info("declared exchange - %s", binding.Exchange)
				} else {
					FailExit("%v", err)
				}

				Info("declaring queue - %s", binding.Queue)
				queue, err := channel.QueueDeclare(
					binding.Queue, // name of the queue
					true,      // durable
					false,     // delete when usused
					false,     // exclusive
					false,     // noWait
					nil,       // arguments
				)
				if err == nil {
					Info("declared queue - %s", binding.Queue)
				} else {
					FailExit("%v", err)
				}

				Info("binding to exchange key - \"%s\"", binding.Routing)
				if err = channel.QueueBind(
					queue.Name,       // name of the queue
					binding.Routing,  // bindingKey
					binding.Exchange, // sourceExchange
					false,            // noWait
					nil,              // arguments
				)
					err == nil {
					Info("queue bind - %s", binding.Queue)
					deliveries, err := channel.Consume(
						binding.Exchange, // name
						"",               // consumerTag,
						false,            // noAck
						false,            // exclusive
						false,            // noLocal
						false,            // noWait
						nil,              // arguments
					)
					if err == nil {
						if binding.Handlers == 0 {
							binding.Handlers = 1
						}
						for i := 0; i < binding.Handlers; i++ {
							go this.consume(i, deliveries)
						}
					} else {
						FailExit("%v", err)
					}
				} else {
					FailExit("%v", err)
				}
			}
		} else {
			FailExit("%v", err)
		}
	} else {
		FailExit("%v", err)
	}
}

func (this *ConsumerApplication) consume(id int, deliveries <- chan amqp.Delivery) {
	for {
		select {
		case delivery := <- deliveries:
			Info("consumer - %d, delivery - %s", id, delivery.Body)
		}
	}
}

func (this *ConsumerApplication) Close() {
	if this.channel != nil {
		this.channel.Close()
	}
	if this.connect != nil {
		this.connect.Close()
	}
}
