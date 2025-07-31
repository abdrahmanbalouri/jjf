// src/ui/UIManager.js
export class UIManager {
    constructor(app) {
        this.app = app;
    }

    showView(view) {
        document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
        const targetView = document.getElementById(`${view}-view`);
        if (targetView) {
            targetView.classList.add('active');
        } else {
            console.error(`View element for "${view}-view" not found.`);
            return;
        }


        // Specific actions for certain views
        switch (view) {
            case 'posts':
                // Make sure the messages section is visible when in 'posts' view (if it was hidden)
                // This might be better managed by CSS or a dedicated chat button.
                // For now, mirroring what was in your original code:
                const messagesSection = document.getElementById('messages');
                if (messagesSection) {
                    messagesSection.style.display = 'flex'; // Assuming 'messages' is the flex container for chat
                }
                this.app.loadPosts(); // Load posts when navigating to posts view
                break;
            // Add other cases for other views if needed
            case 'login':
            case 'register':
                // Hide messages section when on auth pages
                const messagesSectionAuth = document.getElementById('messages');
                if (messagesSectionAuth) {
                    messagesSectionAuth.style.display = 'none';
                }
                break;
        }
    }

    showAuthenticatedUI() {
        document.getElementById('auth-links')?.classList.add('hidden');
        document.getElementById('user-links')?.classList.remove('hidden');
        // Potentially show user-specific elements like nickname display
        const usernameDisplay = document.getElementById('user-nickname-display'); // Assuming you have an element for this
        if (usernameDisplay && this.app.currentUser) {
            usernameDisplay.textContent = this.app.currentUser.nickname;
        }
    }

    showUnauthenticatedUI() {
        document.getElementById('auth-links')?.classList.remove('hidden');
        document.getElementById('user-links')?.classList.add('hidden');
        // Hide elements specific to authenticated users
        const usernameDisplay = document.getElementById('user-nickname-display');
        if (usernameDisplay) {
            usernameDisplay.textContent = '';
        }
    }
}