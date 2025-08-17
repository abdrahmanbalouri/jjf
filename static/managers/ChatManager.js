export class ChatManager {
    constructor(app) {
        this.app = app;
        this.socket = null; // WebSocket instance belongs here
        this.typingTimeout = null;
        this.earliestMessageTimestamp = null; // Track earliest message for pagination
        this.isLoadingMessages = false; // Prevent multiple simultaneous fetches
        this.id = null
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
                // case 'user_status':
                //     this.updateUserStatus(message.userId, message.isOnline);
                //     break;
                case 'online_users':
                    this.loadUsers();
                    break;
                case 'private_message':
                    this.handlePrivateMessage(message.payload);
                    break;
                // case 'message_confirmation':
                //     this.handleMessageConfirmation(message.payload);
                //     break;
                case 'message_read':
                    this.handleMessageRead(message.payload);
                    break;
                case 'typing':
                    this.handleTypingIndicator(message.payload);
                    break;
                case 'stop_typing':
                    this.handleStopTyping(message.payload);
                    break;
                case 'eroor':
                    let b = document.getElementById('not')
                    b.textContent = message.payload.eroor
                    b.classList.add('show');

                    this.id = setTimeout(() => {

                        b.textContent = ""
                        not.classList.remove('show');


                    }, 2000)

                    break
            }
        };
        this.socket.onclose = () => {
            console.log('WebSocket disconnected');
            // this.app.currentUser = null;

            const typingIndicator = document.getElementById('typing-indicator');

            console.log(typingIndicator);

            typingIndicator.textContent = '';





        };
        this.socket.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }
    getCookie(name) {
        return document.cookie
            .split('; ')
            .find(row => row.startsWith(name + '='))
            ?.split('=')[1] || null;
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
        console.log(users);

        const container = document.getElementById('users-list');
        if (!container) return;
        container.innerHTML = users
            .filter(user => user.id !== this.app.currentUser.id)
            .map(user => `
                <div class="user ${user.isOnline ? 'online' : 'offline'}"  data-user-id="${user.id}"
           data-role="${user.nickname}" >
                    <span class="status ${user.isOnline ? 'online' : 'offline'}"></span>
                    ${user.nickname}
                </div>
            `).join('');
        document.querySelectorAll('.user[data-user-id]').forEach(item => {
            item.addEventListener('click', () => {
                console.log(item);

                const userId = item.dataset.userId;
                const userName = item.dataset.role;


                this.startConversation(userId, userName);
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

    async startConversation(userId, userName) {
        this.app.initWebSocket()
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
        document.getElementById('receiver-name').textContent = `${userName}`;
        document.getElementById('conversation-panel').classList.remove('hidden');
        document.getElementById('message-content').focus();
        const backButton = document.getElementById('back-to-users');
        if (backButton) {
            backButton.addEventListener('click', () => {
                this.closeConversation();
            });
        }



        // Load initial 10 messages
        await this.loadMessages(userId);
        await this.markMessagesAsRead(userId);
        const newForm = form.cloneNode(true);
        form.parentNode.replaceChild(newForm, form);
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


                if (messagesContainer.scrollTop <= 100 && !this.isLoadingMessages) {
                    this.loadMoreMessages(userId);
                }


            }, 2000);
        }
    }
    closeConversation() {
        // Cacher le panneau de conversation et réafficher la liste des utilisateurs
        document.getElementById('conversation-panel').classList.add('hidden');

        this.app.currentConversation = null;
        document.getElementById('message-content').value = '';
    }

    async loadMessages(userId, beforeTimestamp = null) {

        try {
            this.isLoadingMessages = true;
            let url = `/api/messages?with=${userId}&limit=10`;
            if (beforeTimestamp) {

                url += `&before=${encodeURIComponent(beforeTimestamp)}`;
            }

            const response = await fetch(url);
            const messages = await response.json();
            const container = document.getElementById('messages-container');

            if (!messages) return
            if (!container) return;
            // If no beforeTimestamp (initial load), clear container
            if (!beforeTimestamp) {
                container.innerHTML = '';
            }

            if (messages.length > 0) {
                console.log(messages[0].timestamp, '-----');

                this.earliestMessageTimestamp = messages[0].timestamp;
            } else {
                return
            }


            // Prepend messages for older messages, append for initial load
            const messageHtml = messages.map(message => `
                <div class="message ${message.senderId === this.app.currentUser.id ? 'sent' : 'received'}" data-message-id="${message.id}">
                    <div class="message-meta">
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
                container.scrollTop = container.scrollHeight;
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
        messagesContainer.scrollTop = messagesContainer.scrollHeight - oldScrollHeight + oldScrollTop;
    }

   async handleTypingIndicator(payload) {
      const token = this.getCookie('session_id');
        try{

            const response = await fetch(`/api/auto?with=${token}`);
              if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();
            console.log(id,'--------------------------');
            
                 
              if (id !== this.app.currentUser.id) {
             if (this.app.socket) {
                this.app.socket.close(); 
            }
           this.app.authManager.handleLogout()
            return
        }
        }catch (err){
           console.log(err);
           
        }
        const typingIndicator = document.getElementById('typing-indicator');
        if (typingIndicator && payload.senderId === this.app.currentConversation) {
            typingIndicator.textContent = `${payload.senderName} is typing`;
        }
    }

    async handleStopTyping(payload) {
        const token = this.getCookie('session_id');
        try{

            const response = await fetch(`/api/auto?with=${token}`);
              if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();
            console.log(id,'--------------------------');
            
                 
              if (id !== this.app.currentUser.id) {
             if (this.app.socket) {
                this.app.socket.close(); 
            }
           this.app.authManager.handleLogout()
            return
        }
        }catch (err){
           console.log(err);
           
        }
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


        const token = this.getCookie('session_id');
        try{

            const response = await fetch(`/api/auto?with=${token}`);
              if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();
            console.log(id,'--------------------------');
            
                 
              if (id !== this.app.currentUser.id) {
             if (this.app.socket) {
                this.app.socket.close(); 
            }
           this.app.authManager.handleLogout()
            return
        }
        }catch (err){
           console.log(err);
           
        }

        const content = document.getElementById('message-content').value;
        const clientMessageId = Date.now().toString() + Math.random().toString(36).substr(2, 9);


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

    }

    async handlePrivateMessage(payload) {
      const token = this.getCookie('session_id');
        try{

            const response = await fetch(`/api/auto?with=${token}`);
              if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();
            console.log(id,'--------------------------');
            
                 
              if (id !== this.app.currentUser.id) {
             if (this.app.socket) {
                this.app.socket.close(); 
            }
           this.app.authManager.handleLogout()
            return
        }
        }catch (err){
           console.log(err);
           
        }

      

        if (payload.senderId == this.app.currentUser.id && payload.receiverId == this.app.currentConversation) {
            console.log(1111);

            this.renderMessage(payload);



        } else {



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
                clearTimeout(this.id)
                let b = document.getElementById('not')
                b.textContent = 'new message' + "     from   " + payload.senderName
                b.classList.add('show');

                this.id = setTimeout(() => {

                    b.textContent = ""
                    not.classList.remove('show');


                }, 2000)

                this.loadUsers();
            }
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

   async handleMessageRead(payload) {
       const token = this.getCookie('session_id');
        try{

            const response = await fetch(`/api/auto?with=${token}`);
              if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();
            console.log(id,'--------------------------');
            
                 
              if (id !== this.app.currentUser.id) {
             if (this.app.socket) {
                this.app.socket.close(); 
            }
           this.app.authManager.handleLogout()
            return
        }
        }catch (err){
           console.log(err);
           
        }
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