package bot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
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
		if err := client.Auth().IfNecessary(ctx, b.authFlow()); err != nil {
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

// TerminalAuth implements auth.UserAuthenticator
type TerminalAuth struct{}

func (TerminalAuth) Phone(_ context.Context) (string, error) {
	fmt.Print("Enter phone number: ")
	reader := bufio.NewReader(os.Stdin)
	phone, _ := reader.ReadString('\n')
	return strings.TrimSpace(phone), nil
}

func (TerminalAuth) Password(_ context.Context) (string, error) {
	fmt.Print("Enter password: ")
	reader := bufio.NewReader(os.Stdin)
	pwd, _ := reader.ReadString('\n')
	return strings.TrimSpace(pwd), nil
}

func (TerminalAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (TerminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter code: ")
	reader := bufio.NewReader(os.Stdin)
	code, _ := reader.ReadString('\n')
	return strings.TrimSpace(code), nil
}

func (TerminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("signup not supported")
}

func (b *Bot) authFlow() auth.Flow {
	return auth.NewFlow(TerminalAuth{}, auth.SendCodeOptions{})
}

func (b *Bot) createDispatcher() telegram.UpdateHandler {
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
			default:
				// Ignore DMs if strict
				if b.config.TargetGroupID != 0 {
					return nil
				}
			}

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
				// Attempt to check replied message sender
				// This requires fetching or context.
				// For now, we only support mentions robustly or if we can fetch easily.
				// We'll skip complex fetching to keep it simple and safe.
				// If user wants to support replies, we can add it later.
				// Just keeping previous logic structure.
				_ = header
			}
		}

		if shouldReply {
			// Resolve peer
			var inputPeer tg.InputPeerClass
			switch p := msg.PeerID.(type) {
			case *tg.PeerUser:
				if user, ok := e.Users[p.UserID]; ok {
					inputPeer = user.AsInputPeer()
				}
			case *tg.PeerChat:
				if chat, ok := e.Chats[p.ChatID]; ok {
					// Chats don't use AccessHash in InputPeerChat
					_ = chat
					inputPeer = &tg.InputPeerChat{ChatID: p.ChatID}
				}
			case *tg.PeerChannel:
				if channel, ok := e.Channels[p.ChannelID]; ok {
					inputPeer = channel.AsInputPeer()
				}
			}

			if inputPeer != nil {
				go b.handleMessage(context.WithoutCancel(ctx), inputPeer, msg)
			} else {
				log.Printf("Could not resolve peer for message %d", msg.ID)
			}
		}

		return nil
	})
	return dispatcher
}

func (b *Bot) handleMessage(ctx context.Context, peer tg.InputPeerClass, msg *tg.Message) {
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
			if p, ok := photo.Photo.(*tg.Photo); ok {
				// Construct InputPhotoFileLocation
				// We need to find the best size.
				// Usually, the last size is the largest.
				if len(p.Sizes) > 0 {
					// Simply pick the last one (usually largest)
					// Or find type 'y' or 'w'.
					// For simplicity, we assume the last one is good enough.
					// Note: InputPhotoFileLocation needs thumb_size.
					// We'll use the type of the last size.
					sizeType := p.Sizes[len(p.Sizes)-1].GetType()

					loc := &tg.InputPhotoFileLocation{
						ID:            p.ID,
						AccessHash:    p.AccessHash,
						FileReference: p.FileReference,
						ThumbSize:     sizeType,
					}

					d := downloader.NewDownloader()
					var buf bytes.Buffer
					_, err := d.Download(b.client.API(), loc).Stream(ctx, &buf)
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
	// Use Sender.To(Peer).Reply(MsgID)
	_, err = b.sender.To(peer).Reply(msg.ID).StyledText(ctx, html.String(nil, resp))
	if err != nil {
		log.Printf("Reply error: %v", err)
	}
}
