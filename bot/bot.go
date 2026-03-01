package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"realMe/config"
	"realMe/llm"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"
)

type Bot struct {
	client    *telegram.Client
	config    *config.Config
	glmClient *llm.GLMClient
	me        *tg.User
	sender    *message.Sender
}

func NewBot(cfg *config.Config, glm *llm.GLMClient) *Bot {
	return &Bot{
		config:    cfg,
		glmClient: glm,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	// Initialize client with file session storage
	// This will save the session to "session.json" in the current directory
	// so you don't have to log in every time.

	client, err := telegram.ClientFromEnvironment(telegram.Options{
		SessionStorage: &session.FileStorage{
			Path: "session.json",
		},
		UpdateHandler: b.createDispatcher(),
	})
	if err != nil {
		client = telegram.NewClient(b.config.TelegramAppID, b.config.TelegramAppHash, telegram.Options{
			SessionStorage: &session.FileStorage{
				Path: "session.json",
			},
			UpdateHandler: b.createDispatcher(),
		})
	}
	b.client = client

	return client.Run(ctx, func(ctx context.Context) error {
		// Auth flow
		if err := client.Auth().IfNeeded(ctx, b.authFlow()); err != nil {
			return err
		}

		// Get self
		me, err := client.Self(ctx)
		if err != nil {
			return err
		}
		b.me = me
		b.sender = message.NewSender(client.API())

		log.Printf("Logged in as %s (@%s)", me.FirstName, me.Username)
		log.Printf("Monitoring group ID: %d", b.config.TargetGroupID)

		// Block until context is canceled
		<-ctx.Done()
		return ctx.Err()
	})
}

func (b *Bot) authFlow() auth.Flow {
	return auth.NewFlow(
		auth.Terminal{
			PhoneNumber: func(ctx context.Context) (string, error) {
				fmt.Print("Enter phone number: ")
				var phone string
				fmt.Scanln(&phone)
				return strings.TrimSpace(phone), nil
			},
			Code: func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
				fmt.Print("Enter code: ")
				var code string
				fmt.Scanln(&code)
				return strings.TrimSpace(code), nil
			},
			Password: func(ctx context.Context) (string, error) {
				fmt.Print("Enter password: ")
				var pwd string
				fmt.Scanln(&pwd)
				return strings.TrimSpace(pwd), nil
			},
		},
		auth.SendCodeOptions{},
	)
}

func (b *Bot) createDispatcher() tg.UpdateHandler {
	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}

		// Check outgoing messages (ignore them)
		if msg.Out {
			return nil
		}

		// Check if it's from the target group
		if b.config.TargetGroupID != 0 {
			var chatID int64
			switch peer := msg.PeerID.(type) {
			case *tg.PeerChannel:
				chatID = peer.ChannelID
			case *tg.PeerChat:
				chatID = peer.ChatID
			// PeerUser is for DMs, maybe allow DMs too?
			// The requirement says "specified group".
			// So if it's a DM, we might ignore it unless we want to support DMs.
			// Let's strictly follow "specified group" if configured.
			default:
				if b.config.TargetGroupID != 0 {
					return nil
				}
			}

			// Simple check: if configured and ID doesn't match
			// Note: This simple check assumes raw ID matches.
			// Telegram IDs can be confusing.
			if b.config.TargetGroupID != 0 && chatID != b.config.TargetGroupID {
				return nil
			}
		}

		// Check triggers
		shouldReply := false

		// 1. Mention @me
		for _, entity := range msg.Entities {
			switch ent := entity.(type) {
			case *tg.MessageEntityMention:
				// ent.Offset, ent.Length
				// Check bounds
				if int(ent.Offset)+int(ent.Length) <= len(msg.Message) {
					mention := msg.Message[ent.Offset : ent.Offset+ent.Length]
					if strings.EqualFold(mention, "@"+b.me.Username) {
						shouldReply = true
					}
				}
			case *tg.MessageEntityMentionName:
				if ent.UserID == b.me.ID {
					shouldReply = true
				}
			}
		}

		// 2. Reply to me
		if !shouldReply && msg.ReplyTo != nil {
			if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok {
				// We need to fetch the message
				// Use sender.Resolve()... no, we need to fetch messages.
				// client.API().MessagesGetMessages
				// But that's expensive for every message.
				// Optimization: Check if ReplyToMsgID is in cache? No cache.
				// Just fetch it.

				// However, fetching requires access hash or input message.
				// In channels, ID is enough?
				// InputMessageID is sufficient for channels if we are in it?
				// Actually, MessagesGetMessages takes InputMessageClass.
				// We need InputChannel if it's a channel.

				// Let's try to get the replied message
				// This part is tricky without context of the channel.
				// We'll skip complex reply check for now and focus on mentions if it's too hard,
				// but let's try a best effort.
				// If we are in a channel, we need to provide the channel input.

				// For simplicity, let's assume we reply if mentioned.
				// If the user REALLY needs "reply to me", we must implement it.
				// "reply to me" means the message being replied to was sent by me.

				// Let's try to fetch the message.
				// We need the input channel.
				var inputChannel *tg.InputChannel
				if peer, ok := msg.PeerID.(*tg.PeerChannel); ok {
					// We need access hash. e.Entities has it?
					// e.Channels has the channel info.
					if ch, ok := e.Channels[peer.ChannelID]; ok {
						inputChannel = &tg.InputChannel{
							ChannelID:  ch.ID,
							AccessHash: ch.AccessHash,
						}
					}
				}

				if inputChannel != nil {
					msgs, err := b.client.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
						Channel: inputChannel,
						ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: header.ReplyToMsgID}},
					})
					if err == nil {
						if channelMsgs, ok := msgs.(*tg.MessagesChannelMessages); ok {
							for _, m := range channelMsgs.Messages {
								if mMsg, ok := m.(*tg.Message); ok {
									// Check if sender is me
									// For messages in channels, FromID might be nil (if sent as channel)
									// or PeerUser.
									if mMsg.FromID != nil {
										if peerUser, ok := mMsg.FromID.(*tg.PeerUser); ok {
											if peerUser.UserID == b.me.ID {
												shouldReply = true
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		if shouldReply {
			// Launch goroutine to handle
			go b.handleMessage(context.WithoutCancel(ctx), msg)
		}

		return nil
	})
	return dispatcher
}

func (b *Bot) handleMessage(ctx context.Context, msg *tg.Message) {
	log.Printf("Replying to message %d", msg.ID)

	// Prepare content
	var contentItems []llm.ContentItem
	contentItems = append(contentItems, llm.ContentItem{
		Type: "text",
		Text: msg.Message,
	})

	// Check for photo
	if msg.Media != nil {
		if photo, ok := msg.Media.(*tg.MessageMediaPhoto); ok {
			// Download
			d := downloader.NewDownloader()
			var buf bytes.Buffer
			_, err := d.Download(b.client.API(), photo.Photo).Stream(ctx, &buf)
			if err != nil {
				log.Printf("Download error: %v", err)
			} else {
				encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
				contentItems = append(contentItems, llm.ContentItem{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: "data:image/jpeg;base64," + encoded,
					},
				})
			}
		}
	}

	// Call GLM
	resp, err := b.glmClient.Chat([]llm.Message{
		{
			Role:    "user",
			Content: contentItems,
		},
	})
	if err != nil {
		log.Printf("GLM Chat error: %v", err)
		return
	}

	// Reply
	_, err = b.sender.Reply(msg).StyledText(ctx, html.String(nil, resp))
	if err != nil {
		log.Printf("Reply error: %v", err)
	}
}
