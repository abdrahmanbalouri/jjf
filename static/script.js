class ForumApp {
    constructor() {
        this.socket = null;
        this.currentUser = null;
        this.currentConversation = null;
        this.pendingMessages = new Map(); // Track pending messages
        this.initialize();
    }

    initialize() {
        this.setupEventListeners();
        this.checkSession();
    }

    setupEventListeners() {
        // Auth navigation
        document.getElementById('nav-register')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('register');
        });

        document.getElementById('nav-login')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('login');
        });

        document.getElementById('show-register')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('register');
        });

        document.getElementById('show-login')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('login');
        });

        // Auth forms
        document.getElementById('login-form')?.addEventListener('submit', (e) => this.handleLogin(e));
        document.getElementById('register-form')?.addEventListener('submit', (e) => this.handleRegister(e));
        document.getElementById('nav-logout')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.handleLogout();
        });

        // Forum navigation
        document.getElementById('nav-posts')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('posts');
        });

        document.getElementById('nav-messages')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.showView('messages');
        });

        // Post form
        document.getElementById('post-form')?.addEventListener('submit', (e) => this.handlePostCreate(e));
    }

    async checkSession() {
        try {
            const response = await fetch('/api/users/me');
            if (response.ok) {
                this.currentUser = await response.json();
                this.showAuthenticatedUI();
                this.initWebSocket();
                this.showView('posts');
            } else {
                this.showUnauthenticatedUI();
                this.showView('login');
            }
        } catch (error) {
            console.error('Session check failed:', error);
            this.showUnauthenticatedUI();
            this.showView('login');
        }
    }

    initWebSocket() {
        this.socket = new WebSocket('ws://localhost:8080/ws');

        this.socket.onopen = () => {
            console.log('WebSocket connected');
        };
          //    this.loadUsers()
        this.socket.onmessage = (event) => {
            if (!event.data) return;
            const message = JSON.parse(event.data);

            switch (message.type) {
                case 'user_status':
                    this.updateUserStatus(message.userId, message.isOnline);
                    break;
                case 'online_users':
                    this.loadUsers
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
            }
        };

        this.socket.onclose = () => {
            console.log('WebSocket disconnected');
            this.currentUser = null;
            this.showUnauthenticatedUI();
            this.showView('login');
        };

        this.socket.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
    }

    showView(view) {
        document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
        document.getElementById(`${view}-view`).classList.add('active');

        switch (view) {
            case 'posts':
                this.loadPosts();
                break;
            case 'messages':
                this.loadUsers();
                break;
        }
    }

    showAuthenticatedUI() {
        document.getElementById('auth-links')?.classList.add('hidden');
        document.getElementById('user-links')?.classList.remove('hidden');
    }

    showUnauthenticatedUI() {
        document.getElementById('auth-links')?.classList.remove('hidden');
        document.getElementById('user-links')?.classList.add('hidden');
    }

    // Auth Methods
    async handleLogin(e) {
        e.preventDefault();
        const identifier = document.getElementById('login-identifier').value;
        const password = document.getElementById('login-password').value;

        try {
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ identifier, password }),
            });

            if (response.ok) {
                const data = await response.json();
                this.currentUser = data.user;
                this.initWebSocket();
                this.showAuthenticatedUI();
                this.showView('posts');
                document.getElementById('login-error').textContent = '';
            } else {
                const error = await response.json();
                document.getElementById('login-error').textContent = error.error || 'Login failed';
            }
        } catch (error) {
            console.error('Login error:', error);
            document.getElementById('login-error').textContent = 'Network error during login';
        }
    }

    async handleRegister(e) {
        e.preventDefault();
        const formData = {
            nickname: document.getElementById('register-nickname').value,
            email: document.getElementById('register-email').value,
            password: document.getElementById('register-password').value,
            age: parseInt(document.getElementById('register-age').value),
            gender: document.getElementById('register-gender').value,
            first_name: document.getElementById('register-first-name').value,
            last_name: document.getElementById('register-last-name').value,
        };

        try {
            const response = await fetch('/api/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData),
            });

            if (response.ok) {
                document.getElementById('register-error').textContent = 'Registration successful! Please login.';
                document.getElementById('register-error').className = 'success';
                this.showView('login');
            } else {
                const error = await response.json();
                document.getElementById('register-error').textContent = error.error || 'Registration failed';
                document.getElementById('register-error').className = 'error';
            }
        } catch (error) {
            console.error('Register error:', error);
            document.getElementById('register-error').textContent = 'Network error during registration';
            document.getElementById('register-error').className = 'error';
        }
    }

    async handleLogout() {
        try {
            const response = await fetch('/api/logout', { method: 'POST' });
            if (response.ok) {
                this.currentUser = null;
                if (this.socket) {
                    this.socket.close();
                }
                this.pendingMessages.clear();
                this.showUnauthenticatedUI();
                this.showView('login');
            } else {
                alert('Logout failed');
            }
        } catch (error) {
            console.error('Logout error:', error);
            alert('Network error during logout');
        }
    }

    // Post Methods
    async loadPosts() {
        try {
            const response = await fetch('/api/posts');
            if (!response.ok) throw new Error('Failed to load posts');
            const posts = await response.json();
            this.renderPosts(posts || []);
        } catch (error) {
            console.error('Error loading posts:', error);
            document.getElementById('posts-container').innerHTML =
                '<div class="error">Failed to load posts. Please try again later.</div>';
        }
    }

    async handlePostCreate(e) {
        e.preventDefault();
        const title = document.getElementById('post-title').value;
        const content = document.getElementById('post-content').value;
        const category = document.getElementById('post-category').value;

        if (!title || !content || !category) {
            alert('Please fill all fields');
            return;
        }

        try {
            const response = await fetch('/api/posts/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ title, content, category }),
            });

            if (response.ok) {
                document.getElementById('post-form').reset();
                this.loadPosts();
            } else {
                const error = await response.json();
                alert(error.error || 'Failed to create post');
            }
        } catch (error) {
            console.error('Error creating post:', error);
            alert('Network error while creating post');
        }
    }

    renderPosts(posts) {
        const container = document.getElementById('posts-container');
        if (!posts || !Array.isArray(posts)) {
            container.innerHTML = '<div class="error">No posts available</div>';
            return;
        }

        if (posts.length === 0) {
            container.innerHTML = '<div>No posts yet. Be the first to post!</div>';
            return;
        }

        container.innerHTML = posts.map(post => `
            <div class="post" data-id="${post.id}">
                <h3 class="post-title">${post.title}</h3>
                <div class="post-meta">
                    <span>Posted by ${post.author || 'Unknown'} in ${post.category || 'General'}</span>
                    <span>${post.created_at ? new Date(post.created_at).toLocaleString() : ''}</span>
                </div>
                <div class="post-content">${post.content || ''}</div>
                <button class="view-comments" data-post-id="${post.id}">
                    View Comments
                </button>
            </div>
        `).join('');

        document.querySelectorAll('.view-comments').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const postId = e.target.dataset.postId;
                this.showCommentPopup(postId);
            });
        });
    }

    async showCommentPopup(postId) {
        try {
            const [postResponse, commentsResponse] = await Promise.all([
                fetch(`/api/posts/${postId}`),
                fetch(`/api/comments?post_id=${postId}`)
            ]);

            if (!postResponse.ok) throw new Error('Failed to load post');
            if (!commentsResponse.ok) throw new Error('Failed to load comments');

            const post = await postResponse.json();
            const comments = await commentsResponse.json();

            document.getElementById('popup-post-title').textContent = post.title || 'Post';
            document.getElementById('popup-comment-form').dataset.postId = postId;
            this.renderComments(comments || [], 'popup-comments-container');

            const popup = document.getElementById('comment-popup');
            popup.classList.remove('hidden');

            const closeBtn = document.getElementById('popup-close');
            closeBtn.onclick = () => popup.classList.add('hidden');

            const form = document.getElementById('popup-comment-form');
            console.log('Form element:', form); // Debug: Check if form is found
            if (!form) {
                console.error('Popup comment form not found!');
                return;
            }
            // Remove existing listeners to prevent duplicates
            const newForm = form.cloneNode(true);
            form.parentNode.replaceChild(newForm, form);
            newForm.addEventListener('submit', (e) => {
                this.handleCommentCreate(e);
            });
        } catch (error) {
            console.error('Error showing comment popup:', error);
            document.getElementById('popup-post-title').textContent = 'Error loading post';
            document.getElementById('popup-comments-container').innerHTML =
                '<div class="error">Failed to load post details. Please try again.</div>';
        }
    }

    // Comment Methods
    async handleCommentCreate(e) {
        e.preventDefault();
        const content = document.getElementById('popup-comment-content').value;
        const postId = e.target.dataset.postId;

        console.log('Creating comment:', { postId, content }); // Debug: Log form data

        if (!content || !postId) {
            alert('Please enter a comment');
            return;
        }

        try {
            const response = await fetch('/api/comments', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ post_id: postId, content }),
            });

            if (response.ok) {
                e.target.reset();
                this.loadComments(postId, 'popup-comments-container');
            } else {
                const error = await response.json();
                alert(error.error || 'Failed to create comment');
            }
        } catch (error) {
            console.error('Error creating comment:', error);
            alert('Network error while creating comment');
        }
    }

    renderComments(comments, containerId = 'popup-comments-container') {
        const container = document.getElementById(containerId);
        container.innerHTML = comments.map(comment => `
            <div class="comment">
                <div class="comment-meta">
                    <span>${comment.author}</span>
                    <span>${new Date(comment.created_at).toLocaleString()}</span>
                </div>
                <div class="comment-content">${comment.content}</div>
            </div>
        `).join('');
    }

    async loadComments(postId, containerId = 'popup-comments-container') {
        try {
            const response = await fetch(`/api/comments?post_id=${postId}`);
            if (!response.ok) throw new Error('Failed to load comments');
            const comments = await response.json();
            this.renderComments(comments, containerId);
        } catch (error) {
            console.error('Error loading comments:', error);
            document.getElementById(containerId).innerHTML =
                '<div class="error">Failed to load comments</div>';
        }
    }

    // Message Methods
    async loadUsers() {
        try {
            const response = await fetch('/api/users');
            if (!response.ok) throw new Error('Failed to load users');
            const users = await response.json();

            const usersWithMessages = await Promise.all(
                users
                    .filter(user => user.id !== this.currentUser.id)
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
            .filter(user => user.id !== this.currentUser.id)
            .map(user => `
                <div class="user ${user.isOnline ? 'online' : 'offline'}" data-user-id="${user.id}">
                    <span class="status ${user.isOnline ? 'online' : 'offline'}"></span>
                    ${user.nickname}
                </div>
            `).join('');

        document.querySelectorAll('.user').forEach(item => {
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
        this.currentConversation = userId;
        document.getElementById('message-form').dataset.userId = userId;
        await this.loadMessages(userId);

        await this.markMessagesAsRead(userId);

        const form = document.getElementById('message-form');
        const newForm = form.cloneNode(true);
        form.parentNode.replaceChild(newForm, form);
        newForm.addEventListener('submit', (e) => {
            e.preventDefault();
            this.sendMessage(userId);
        });
    }

    async loadMessages(userId) {
        try {
            const response = await fetch(`/api/messages?with=${userId}`);
            if (!response.ok) throw new Error('Failed to load messages');
            const messages = await response.json();
            const container = document.getElementById('messages-container');
            if (!container) return;

            container.innerHTML = messages.map(message => `
                <div class="message ${message.senderId === this.currentUser.id ? 'sent' : 'received'}" data-message-id="${message.id}">
                    <div class="message-meta">
                        <span>${message.sender}</span>
                        <span>${new Date(message.timestamp).toLocaleString()}</span>
                        ${message.senderId === this.currentUser.id ? `<span class="read-status">${message.isRead ? '✓✓' : '✓'}</span>` : ''}
                    </div>
                    <div class="message-content">${message.content}</div>
                </div>
            `).join('');

            this.pendingMessages.forEach((msg, clientMessageId) => {
                if (msg.receiverId === userId) {
                    const messageElement = `
                        <div class="message sent" data-message-id="${clientMessageId}">
                            <div class="message-meta">
                                <span>${this.currentUser.nickname}</span>
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

        if (this.currentConversation === receiverId) {
            this.renderMessage({
                messageId: clientMessageId,
                senderId: this.currentUser.id,
                senderName: this.currentUser.nickname,
                content: content,
                timestamp: timestamp,
                isRead: false,
            });
        }

        this.pendingMessages.set(clientMessageId, {
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
            this.loadUsers()
            document.getElementById('message-content').value = '';
        } catch (error) {
            console.error('Error sending message:', error);
            alert('Failed to send message');
            this.pendingMessages.delete(clientMessageId);
            const messageElement = document.querySelector(`.message[data-message-id="${clientMessageId}"]`);
            if (messageElement) messageElement.remove();
        }
    }

    handlePrivateMessage(payload) {
        if (this.currentConversation && payload.senderId === this.currentConversation) {
            this.renderMessage(payload);
            this.socket.send(JSON.stringify({
                type: 'mark_read',
                payload: {
                    senderId: payload.senderId,
                    messageId: payload.messageId,
                },
            }));
        } else {
            console.log(`New message from ${payload.senderName}`);
            this.loadUsers();
        }
    }

    handleMessageConfirmation(payload) {
        const clientMessageId = payload.clientMessageId;
        if (this.pendingMessages.has(clientMessageId)) {
            const oldMessageElement = document.querySelector(`.message[data-message-id="${clientMessageId}"]`);
            if (oldMessageElement && this.currentConversation === payload.receiverId) {
                oldMessageElement.setAttribute('data-message-id', payload.messageId);
                const readStatus = oldMessageElement.querySelector('.read-status');
                if (readStatus) {
                    readStatus.textContent = payload.isRead ? '✓✓' : '✓';
                }
            }
            this.pendingMessages.delete(clientMessageId);
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
            <div class="message ${message.senderId === this.currentUser.id ? 'sent' : 'received'}" data-message-id="${message.messageId}">
                <div class="message-meta">
                    <span>${message.senderName}</span>
                    <span>${new Date(message.timestamp).toLocaleString()}</span>
                    ${message.senderId === this.currentUser.id ? `<span class="read-status">${message.isRead ? '✓✓' : '✓'}</span>` : ''}
                </div>
                <div class="message-content">${message.content}</div>
            </div>
        `;
        container.insertAdjacentHTML('beforeend', messageElement);
        container.scrollTop = container.scrollHeight;
    }
}

document.addEventListener('DOMContentLoaded', () => {
    window.forumApp = new ForumApp();
});