// src/ForumApp.js
import { AuthManager } from './managers/AuthManager.js';
import { PostManager } from './managers/PostManager.js';
import { ChatManager } from './managers/ChatManager.js';
import { UIManager } from './ui/UIManager.js';

export class ForumApp {
    constructor() {
        this.socket = null;
        this.currentUser = null;
        this.currentConversation = null;

        // Initialize managers and pass a reference to this ForumApp instance
        this.uiManager = new UIManager(this);
        this.authManager = new AuthManager(this);
        this.postManager = new PostManager(this);
        this.chatManager = new ChatManager(this);

        this.initialize();
    }

    initialize() {
        // Event listeners related to app-level navigation (e.g., login/register links)
     //   this.authManager.setupAuthEventListeners();
       // this.postManager.setupPostEventListeners(); // For the 'post-form'
        // Note: Chat listeners are often set up dynamically when a conversation starts or users load

        this.authManager.checkSession(); // Start by checking user session
    }

    // --- Core Methods delegated to Managers ---
    showView(view) {
        this.uiManager.showView(view);
    }

    showAuthenticatedUI() {
        this.uiManager.showAuthenticatedUI();
    }

    showUnauthenticatedUI() {
        this.uiManager.showUnauthenticatedUI();
    }

    // Methods for WebSocket initialization, handled by ChatManager
    initWebSocket() {
        this.chatManager.initWebSocket();
    }

    // Methods that need to be globally accessible by managers
    // (e.g., AuthManager calling loadUsers from ChatManager)
    loadUsers() {
        this.chatManager.loadUsers();
    }

    loadPosts() {
        this.postManager.loadPosts();
    }

    // You might add other global methods here if multiple managers need to call them
    // For example, if a logout from AuthManager needs to clear chat messages.
    clearChatMessages() {
        this.chatManager.clearMessages(); // A new method you might add to ChatManager
    }
}