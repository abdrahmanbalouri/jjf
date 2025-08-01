export class ChatManager {
    constructor(app) {
        this.app = app;
        this.socket = null; // WebSocket instance belongs here
        this.typingTimeout = null;
        this.earliestMessageTimestamp = null; // Track earliest message for pagination
        this.isLoadingMessages = false; // Prevent multiple simultaneous fetches
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
            this.loadUsers()
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
            const sortedUsers = usersWithMessages.sort((a, b) => {
                const timeA = a.latestMessage ? new Date(a.latestMessage) : null;
                const timeB = b.latestMessage ? new Date(b.latestMessage) : null;
                if (timeA && timeB) return timeB - timeA;
                if (timeA) return -1;
                if (timeB) return 1;
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
            .filter(user => user.id !== this.app.currentUser.id)
            .map(user => `
                <div class="user ${user.isOnline ? 'online' : 'offline'}" data-user-id="${user.id}">
                    <span class="status ${user.isOnline ? 'online' : 'offline'}"></span>
                    ${user.nickname}
                </div>
            `).join('');
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
        this.app.currentConversation = userId;
        const form = document.getElementById('message-form');
        if (form) {
            form.dataset.userId = userId;
        } else {
            console.error('Message form not found!');
            return;
        }
        const typingIndicator = document.getElementById('typing-indicator');
        if (typingIndicator) {
            typingIndicator.textContent = '';
        }
        // Reset pagination state
        this.earliestMessageTimestamp = null;
        this.isLoadingMessages = false;
        // Load initial 10 messages
        await this.loadMessages(userId);
        await this.markMessagesAsRead(userId);
        const newForm = form.cloneNode(true);
        form.parentNode.replaceChild(newForm, form);
        const messageInput = document.getElementById('message-content');
        if (this.typingTimeout) {
            clearTimeout(this.typingTimeout);
        }
        const newMessageInput = document.getElementById('message-content');
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
        newForm.addEventListener('submit', (e) => {
            e.preventDefault();
            this.sendMessage(userId);
            this.socket.send(JSON.stringify({
                type: 'stop_typing',
                payload: {
                    receiverId: userId,
                },
            }));
        });
        // Add throttled scroll event listener
        const messagesContainer = document.getElementById('messages-container');
        if (messagesContainer) {
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
            // Remove any existing scroll listeners to prevent duplicates
            messagesContainer.onscroll = null;
        
            
         messagesContainer.onscroll = this.throttle(() => {
            
        if (messagesContainer.scrollTop < 50 && !this.isLoadingMessages) {
            this.loadMoreMessages(userId);
        }
    }, 500); 
        }
    }

    async loadMessages(userId, beforeTimestamp = null) {
        try {
            this.isLoadingMessages = true;
            let url = `/api/messages?with=${userId}&limit=10`;
            if (beforeTimestamp) {
                
                url += `&before=${encodeURIComponent(beforeTimestamp)}`;
            }
            
            const response = await fetch(url);
            if (!response.ok) throw new Error('Failed to load messages');
            const messages = await response.json();
            const container = document.getElementById('messages-container');
            if (!container) return;
          
            if (messages.length > 0) {
                this.earliestMessageTimestamp = messages[0].timestamp;
            }
            // Prepend messages for older messages, append for initial load
            const messageHtml = messages.map(message => `
                <div class="message ${message.senderId === this.app.currentUser.id ? 'sent' : 'received'}" data-message-id="${message.id}">
                    <div class="message-meta">
                        <span>${message.sender}</span>
                        <span>${new Date(message.timestamp).toLocaleString()}</span>
                        ${message.senderId === this.app.currentUser.id ? `<span class="read-status">${message.isRead ? '✓✓' : '✓'}</span>` : ''}
                    </div>
                    <div class="message-content">${message.content}</div>
                </div>
            `).join('');
            if (beforeTimestamp) {
                container.insertAdjacentHTML('afterbegin', messageHtml);
            } else {
                container.insertAdjacentHTML('beforeend', messageHtml);
                //container.scrollTop = container.scrollHeight;
            }
        } catch (error) {
            console.error('Error loading messages:', error);
            document.getElementById('messages-container').innerHTML =
                '<div class="error">Failed to load messages</div>';
        } finally {
            this.isLoadingMessages = false;
        }
    }

    async loadMoreMessages(userId) {
        if (!this.earliestMessageTimestamp) return; // No more messages to load
        const messagesContainer = document.getElementById('messages-container');
        const oldScrollHeight = messagesContainer.scrollHeight;
        const oldScrollTop = messagesContainer.scrollTop;
        await this.loadMessages(userId, this.earliestMessageTimestamp);
        // Adjust scroll position to maintain view
     //   messagesContainer.scrollTop = messagesContainer.scrollHeight - oldScrollHeight + oldScrollTop;
    }

    handleTypingIndicator(payload) {
        const typingIndicator = document.getElementById('typing-indicator');
        if (typingIndicator && payload.senderId === this.app.currentConversation) {
            typingIndicator.textContent = `${payload.senderName} is typing...`;
        }
    }

    handleStopTyping(payload) {
        const typingIndicator = document.getElementById('typing-indicator');
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
        if (this.app.currentConversation === receiverId) {
            this.renderMessage({
                messageId: clientMessageId,
                senderId: this.app.currentUser.id,
                senderName: this.app.currentUser.nickname,
                content: content,
                timestamp: timestamp,
                isRead: false,
            });
        }
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
                    messageId: clientMessageId,
                },
            }));
            this.loadUsers();
            document.getElementById('message-content').value = '';
        } catch (error) {
            console.error('Error sending message:', error);
            alert('Failed to send message');
            this.app.pendingMessages.delete(clientMessageId);
            const messageElement = document.querySelector(`.message[data-message-id="${clientMessageId}"]`);
            if (messageElement) messageElement.remove();
        }
    }

    handlePrivateMessage(payload) {
        if (this.app.currentConversation && payload.senderId === this.app.currentConversation) {
            this.renderMessage(payload);
            this.socket.send(JSON.stringify({
                type: 'mark_read',
                payload: {
                    senderId: payload.senderId,
                    messageId: payload.messageId,
                },
            }));
            this.loadUsers();
        } else {
            this.loadUsers();
        }
    }
     throttle(func, wait) {
    let isThrottled = false;
    return function (...args) {
        if (!isThrottled) {
            isThrottled = true;
            func.apply(this, args);
            setTimeout(() => {
                isThrottled = false;
            }, wait);
        }
    };
}

    handleMessageConfirmation(payload) {
        const clientMessageId = payload.clientMessageId;
        if (this.app.pendingMessages.has(clientMessageId)) {
            const oldMessageElement = document.querySelector(`.message[data-message-id="${clientMessageId}"]`);
            if (oldMessageElement && this.app.currentConversation === payload.receiverId) {
                oldMessageElement.setAttribute('data-message-id', payload.messageId);
                const readStatus = oldMessageElement.querySelector('.read-status');
                if (readStatus) {
                    readStatus.textContent = payload.isRead ? '✓✓' : '✓';
                }
            }
            this.app.pendingMessages.delete(clientMessageId);
        }
    }

    handleMessageRead(payload) {
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
        container.scrollTop = container.scrollHeight;
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