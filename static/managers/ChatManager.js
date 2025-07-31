// src/managers/ChatManager.js
export class ChatManager {
    constructor(app) {
        this.app = app;
        this.socket = null; // WebSocket instance belongs here
        this.typingTimeout = null; 
    }

    initWebSocket() {
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            console.log('WebSocket already connected.');
            return;
        }

        this.socket = new WebSocket('ws://localhost:8080/ws');
        this.app.socket = this.socket; // Link the app's socket to this manager's socket

        this.socket.onopen = () => {
            console.log('WebSocket connected');
            this.loadUsers(); // Load users once connected
        };

        this.socket.onmessage = (event) => {
            if (!event.data) return;
            const message = JSON.parse(event.data);

            switch (message.type) {
                case 'user_status':
                    this.updateUserStatus(message.userId, message.isOnline);
                    break;
                case 'online_users':
                    // We call loadUsers here to get the full updated list
                    // which includes online/offline status for all
                    this.loadUsers();
                    break;
                case 'private_message':
                    this.handlePrivateMessage(message.payload);
                    break;
                case 'message_confirmation':
                    this.handleMessageConfirmation(message.payload);
                    break;
                case 'message_read':
                    this.handleMessageRead(message.payload);
                    break;
                case 'typing':
                    this.handleTypingIndicator(message.payload);
                    break;
                case 'stop_typing':
                    this.handleStopTyping(message.payload);
                    break;
            }
        };

        this.socket.onclose = () => {
            console.log('WebSocket disconnected');
            this.app.currentUser = null;
            // Optionally clear chat messages or users list on disconnect
            this.clearMessages();
            document.getElementById('users-list').innerHTML = ''; // Clear user list
            this.app.showUnauthenticatedUI();
            this.app.showView('login');
        };

        this.socket.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }

    async loadUsers() {
        try {
            const response = await fetch('/api/users');
            if (!response.ok) throw new Error('Failed to load users');
            const users = await response.json();

            // Filter out current user and fetch latest message for sorting
            const usersWithMessages = await Promise.all(
                users
                    .filter(user => this.app.currentUser && user.id !== this.app.currentUser.id)
                    .map(async user => {
                        try {
                            const msgResponse = await fetch(`/api/messages?with=${user.id}`);
                            if (!msgResponse.ok) throw new Error('Failed to fetch messages');
                            const messages = await msgResponse.json();
                            const latestMessage = messages
                                .sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp))[0]?.timestamp || null;
                            return { ...user, latestMessage };
                        } catch (error) {
                            console.error(`Error fetching messages for user ${user.id}:`, error);
                            return { ...user, latestMessage: null };
                        }
                    })
            );

            // Sort users: recently chatted first, then online, then alphabetically
            const sortedUsers = usersWithMessages.sort((a, b) => {
                const timeA = a.latestMessage ? new Date(a.latestMessage) : null;
                const timeB = b.latestMessage ? new Date(b.latestMessage) : null;

                if (timeA && timeB) return timeB - timeA; // Sort by latest message
                if (timeA) return -1; // User A has messages, User B doesn't
                if (timeB) return 1;  // User B has messages, User A doesn't

                // If no messages, sort by online status, then nickname
                if (a.isOnline && !b.isOnline) return -1;
                if (!a.isOnline && b.isOnline) return 1;
                return a.nickname.localeCompare(b.nickname);
            });

            this.renderUsers(sortedUsers);
        } catch (error) {
            console.error('Error loading users:', error);
            const container = document.getElementById('users-list');
            if (container) {
                container.innerHTML = '<div class="error">Failed to load users. Please try again.</div>';
            }
        }
    }

    renderUsers(users) {
        const container = document.getElementById('users-list');
        if (!container) return;

        container.innerHTML = users
            .filter(user => user.id !== this.app.currentUser.id) // Ensure current user is not in the list
            .map(user => `
                <div class="user ${user.isOnline ? 'online' : 'offline'}" data-user-id="${user.id}">
                    <span class="status ${user.isOnline ? 'online' : 'offline'}"></span>
                    ${user.nickname}
                </div>
            `).join('');

        // Attach event listeners for each user item to start a conversation
        document.querySelectorAll('.user[data-user-id]').forEach(item => {
            item.addEventListener('click', () => {
                const userId = item.dataset.userId;
                this.startConversation(userId);
            });
        });
    }

    updateUserStatus(userId, isOnline) {
        const userElement = document.querySelector(`.user[data-user-id="${userId}"]`);
        if (userElement) {
            userElement.classList.toggle('online', isOnline);
            userElement.classList.toggle('offline', !isOnline);
            const statusElement = userElement.querySelector('.status');
            if (statusElement) {
                statusElement.classList.toggle('online', isOnline);
                statusElement.classList.toggle('offline', !isOnline);
            }
        }
    }

    async startConversation(userId) {
        // Update the current conversation in the main app instance
        this.app.currentConversation = userId;

        // Update the form's data-user-id attribute
        const form = document.getElementById('message-form');
        if (form) {
            form.dataset.userId = userId;
        } else {
            console.error('Message form not found!');
            return;
        }

        // Clear any existing typing indicator
        const typingIndicator = document.getElementById('typing-indicator');
        if (typingIndicator) {
            typingIndicator.textContent = '';
        }

        // Load messages for the new conversation
        await this.loadMessages(userId);
        await this.markMessagesAsRead(userId);

        // --- IMPORTANT: Re-apply the event listener cleanup here ---
        // This ensures old listeners are removed before new ones are attached
        const newForm = form.cloneNode(true);
        form.parentNode.replaceChild(newForm, form);

        const messageInput = document.getElementById('message-content');
        // Clear any previous typing timeout when starting a new conversation
        if (this.typingTimeout) {
            clearTimeout(this.typingTimeout);
        }

        // Add typing event listener
        // The listener is now attached to the *newForm* element directly,
        // or rather, the messageInput element which is a child of the newForm.
        // We need to re-get messageInput AFTER cloning the form.
        const newMessageInput = document.getElementById('message-content'); // Get the new input element
        newMessageInput.addEventListener('input', () => {
            clearTimeout(this.typingTimeout);
            this.socket.send(JSON.stringify({
                type: 'typing',
                payload: {
                    receiverId: userId,
                },
            }));
            this.typingTimeout = setTimeout(() => {
                this.socket.send(JSON.stringify({
                    type: 'stop_typing',
                    payload: {
                        receiverId: userId,
                    },
                }));
            }, 1000);
        });

        // Add form submit event listener
        newForm.addEventListener('submit', (e) => {
            e.preventDefault();
            this.sendMessage(userId);
            // Ensure stop_typing is sent when message is sent
            this.socket.send(JSON.stringify({
                type: 'stop_typing',
                payload: {
                    receiverId: userId,
                },
            }));
        });
        // Scroll to the bottom of the messages container after loading
        const messagesContainer = document.getElementById('messages-container');
        if (messagesContainer) {
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
    }

    async loadMessages(userId) {
        try {
            const response = await fetch(`/api/messages?with=${userId}`);
            if (!response.ok) throw new Error('Failed to load messages');
            const messages = await response.json();
            const container = document.getElementById('messages-container');
            if (!container) return;

            container.innerHTML = messages.map(message => `
                <div class="message ${message.senderId === this.app.currentUser.id ? 'sent' : 'received'}" data-message-id="${message.id}">
                    <div class="message-meta">
                        <span>${message.sender}</span>
                        <span>${new Date(message.timestamp).toLocaleString()}</span>
                        ${message.senderId === this.app.currentUser.id ? `<span class="read-status">${message.isRead ? '✓✓' : '✓'}</span>` : ''}
                    </div>
                    <div class="message-content">${message.content}</div>
                </div>
            `).join('');

            // Append pending messages (if any)
            this.app.pendingMessages.forEach((msg, clientMessageId) => {
                if (msg.receiverId === userId) {
                    const messageElement = `
                        <div class="message sent" data-message-id="${clientMessageId}">
                            <div class="message-meta">
                                <span>${this.app.currentUser.nickname}</span>
                                <span>${new Date(msg.timestamp).toLocaleString()}</span>
                                <span class="read-status">✓</span>
                            </div>
                            <div class="message-content">${msg.content}</div>
                        </div>
                    `;
                    container.insertAdjacentHTML('beforeend', messageElement);
                }
            });

            container.scrollTop = container.scrollHeight;
        } catch (error) {
            console.error('Error loading messages:', error);
            document.getElementById('messages-container').innerHTML =
                '<div class="error">Failed to load messages</div>';
        }
    }

    handleTypingIndicator(payload) {
        const typingIndicator = document.getElementById('typing-indicator');
        // Only show typing indicator if the typing user is the current conversation partner
        if (typingIndicator && payload.senderId === this.app.currentConversation) {
            typingIndicator.textContent = `${payload.senderName} is typing...`;
        }
    }

    handleStopTyping(payload) {
        const typingIndicator = document.getElementById('typing-indicator');
        // Only clear typing indicator if the user who stopped typing is the current conversation partner
        if (typingIndicator && payload.senderId === this.app.currentConversation) {
            typingIndicator.textContent = '';
        }
    }

    async markMessagesAsRead(senderId) {
        try {
            const response = await fetch(`/api/messages?with=${senderId}`);
            if (!response.ok) throw new Error('Failed to fetch messages');
            const messages = await response.json();

            for (const message of messages) {
                // Only mark as read if it's a message from the sender (not current user)
                // AND it hasn't been read yet
                if (message.senderId === senderId && !message.isRead) {
                    this.socket.send(JSON.stringify({
                        type: 'mark_read',
                        payload: {
                            senderId: senderId,
                            messageId: message.id,
                        },
                    }));
                }
            }
        } catch (error) {
            console.error('Error marking messages as read:', error);
        }
    }

    async sendMessage(receiverId) {
        const content = document.getElementById('message-content').value;
        if (!content) return;

        const clientMessageId = Date.now().toString() + Math.random().toString(36).substr(2, 9);
        const timestamp = new Date().toISOString();

        // Immediately render message if it's for the current conversation
        if (this.app.currentConversation === receiverId) {
            this.renderMessage({
                messageId: clientMessageId,
                senderId: this.app.currentUser.id,
                senderName: this.app.currentUser.nickname,
                content: content,
                timestamp: timestamp,
                isRead: false, // Initially false, will be confirmed by server
            });
        }

        // Store as pending message
        this.app.pendingMessages.set(clientMessageId, {
            receiverId,
            content,
            timestamp,
        });

        try {
            this.socket.send(JSON.stringify({
                type: 'private_message',
                payload: {
                    receiverId,
                    content,
                    messageId: clientMessageId, // Client-side ID for confirmation
                },
            }));
            this.loadUsers(); // Update users list (e.g., for unread counts)
            document.getElementById('message-content').value = ''; // Clear input field
        } catch (error) {
            console.error('Error sending message:', error);
            alert('Failed to send message');
            this.app.pendingMessages.delete(clientMessageId); // Remove from pending
            // Remove the temporarily rendered message if send fails
            const messageElement = document.querySelector(`.message[data-message-id="${clientMessageId}"]`);
            if (messageElement) messageElement.remove();
        }
    }

    handlePrivateMessage(payload) {
        // If the received message is for the currently active conversation
        if (this.app.currentConversation && payload.senderId === this.app.currentConversation) {
            this.renderMessage(payload); // Render it immediately
            // Send read receipt
            this.socket.send(JSON.stringify({
                type: 'mark_read',
                payload: {
                    senderId: payload.senderId,
                    messageId: payload.messageId,
                },
            }));
            this.loadUsers(); // Update users list (e.g., for unread counts, if implemented)
        } else {
            // If the message is not for the current conversation, just update the users list
            // (e.g., to show a new message indicator next to the sender's name)
            this.loadUsers();
        }
    }

    handleMessageConfirmation(payload) {
        const clientMessageId = payload.clientMessageId;
        if (this.app.pendingMessages.has(clientMessageId)) {
            const oldMessageElement = document.querySelector(`.message[data-message-id="${clientMessageId}"]`);
            if (oldMessageElement && this.app.currentConversation === payload.receiverId) {
                oldMessageElement.setAttribute('data-message-id', payload.messageId); // Update with server's message ID
                const readStatus = oldMessageElement.querySelector('.read-status');
                if (readStatus) {
                    readStatus.textContent = payload.isRead ? '✓✓' : '✓'; // Update read status
                }
            }
            this.app.pendingMessages.delete(clientMessageId); // Message confirmed, remove from pending
        }
    }

    handleMessageRead(payload) {
        // Find the message element and update its read status to 'read' (✓✓)
        const messageElement = document.querySelector(`.message[data-message-id="${payload.messageId}"] .read-status`);
        if (messageElement) {
            messageElement.textContent = '✓✓';
        }
    }

    renderMessage(message) {
        const container = document.getElementById('messages-container');
        if (!container) return;

        const messageElement = `
            <div class="message ${message.senderId === this.app.currentUser.id ? 'sent' : 'received'}" data-message-id="${message.messageId}">
                <div class="message-meta">
                    <span>${message.senderName}</span>
                    <span>${new Date(message.timestamp).toLocaleString()}</span>
                    ${message.senderId === this.app.currentUser.id ? `<span class="read-status">${message.isRead ? '✓✓' : '✓'}</span>` : ''}
                </div>
                <div class="message-content">${message.content}</div>
            </div>
        `;
        container.insertAdjacentHTML('beforeend', messageElement);
        container.scrollTop = container.scrollHeight; // Scroll to bottom
    }

    clearMessages() {
        const container = document.getElementById('messages-container');
        if (container) {
            container.innerHTML = '';
        }
        const typingIndicator = document.getElementById('typing-indicator');
        if (typingIndicator) {
            typingIndicator.textContent = '';
        }
    }
}