// src/managers/AuthManager.js
export class AuthManager {
    constructor(app) {
        this.app = app; // Reference to the main ForumApp instance
    }

    setupAuthEventListeners() {
        // Auth navigation
        document.getElementById('nav-register')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.app.showView('register');
        });

        document.getElementById('nav-login')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.app.showView('login');
        });

        document.getElementById('show-register')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.app.showView('register');
        });

        document.getElementById('show-login')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.app.showView('login');
        });

        // Auth forms
        document.getElementById('login-form')?.addEventListener('submit', (e) => this.handleLogin(e));
        document.getElementById('register-form')?.addEventListener('submit', (e) => this.handleRegister(e));
        document.getElementById('nav-logout')?.addEventListener('click', (e) => {
            e.preventDefault();
            this.handleLogout();
        });
    }

    async checkSession() {
        try {
            const response = await fetch('/api/users/me');
            if (response.ok) {
                this.app.currentUser = await response.json();
                this.app.showAuthenticatedUI();
                this.app.initWebSocket(); // Initialize WebSocket only after user is authenticated
                this.app.showView('posts'); // Show posts after login
            } else {
                this.app.showUnauthenticatedUI();
                this.app.showView('login');
            }
        } catch (error) {
            console.error('Session check failed:', error);
            this.app.showUnauthenticatedUI();
            this.app.showView('login');
        }
    }

    async handleLogin(e) {
        e.preventDefault();
        const identifier = document.getElementById('login-identifier').value;
        const password = document.getElementById('login-password').value;
        const loginErrorElement = document.getElementById('login-error');

        try {
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ identifier, password }),
            });

            if (response.ok) {
                const data = await response.json();
                this.app.currentUser = data.user;
                this.app.initWebSocket();
                this.app.showAuthenticatedUI();
                this.app.showView('posts');
                loginErrorElement.textContent = ''; // Clear error on success
                // Make sure 'messages' container is visible if it was hidden
                document.getElementById('messages').style.display = 'block';
            } else {
                const error = await response.json();
                loginErrorElement.textContent = error.error || 'Login failed';
            }
        } catch (error) {
            console.error('Login error:', error);
            loginErrorElement.textContent = 'Network error during login';
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
        const registerErrorElement = document.getElementById('register-error');

        try {
            const response = await fetch('/api/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(formData),
            });

            if (response.ok) {
                registerErrorElement.textContent = 'Registration successful! Please login.';
                registerErrorElement.className = 'success';
                this.app.showView('login');
            } else {
                const error = await response.json();
                registerErrorElement.textContent = error.error || 'Registration failed';
                registerErrorElement.className = 'error';
            }
        } catch (error) {
            console.error('Register error:', error);
            registerErrorElement.textContent = 'Network error during registration';
            registerErrorElement.className = 'error';
        }
    }

    async handleLogout() {
        try {
            const response = await fetch('/api/logout', { method: 'POST' });
            if (response.ok) {
                this.app.currentUser = null;
                if (this.app.socket) {
                    this.app.socket.close(); // Close WebSocket connection
                }
                this.app.pendingMessages.clear(); // Clear pending messages
                this.app.showUnauthenticatedUI();
                this.app.showView('login');
                // Hide messages container on logout
                document.getElementById('messages').style.display = 'none';
            } else {
                alert('Logout failed');
            }
        } catch (error) {
            console.error('Logout error:', error);
            alert('Network error during logout');
        }
    }
}