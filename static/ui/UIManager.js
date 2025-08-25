// src/managers/UIManager.js
import { login, register, posts, messages } from './templates.js';

export class UIManager {
    constructor(app) {
        this.app = app;
    }

    showView(view) {
        console.log(view);

        const appContainer = document.getElementById('app-container');
        appContainer.innerHTML = ''; // Clear previous content
        switch (view) {
            case 'posts':
                appContainer.innerHTML = messages + posts; // Show both posts and messages
                this.app.postManager.setupPostEventListeners();
                this.app.loadPosts();
                this.app.showAuthenticatedUI()
                this.app.loadUsers()
                break;
            case 'login':
                appContainer.innerHTML = login;

                this.deleteCookie("session_id");

                break;
            case 'register':
                appContainer.innerHTML = register;
                break;
                
            }
            this.app.authManager.setupAuthEventListeners(); // Reattach auth listeners
    }
    deleteCookie(name, path = "/") {
        console.log(222);

        document.cookie = name + "=; Max-Age=0; path=" + path;
    }


    showAuthenticatedUI() {
        document.getElementById('auth-links').classList.add('hidden');
        document.getElementById('user-links').classList.remove('hidden');
        const usernameDisplay = document.getElementById('user-nickname-display');
        if (usernameDisplay && this.app.currentUser) {
            usernameDisplay.textContent = this.app.currentUser.nickname;
        }
    }

    showUnauthenticatedUI() {
        document.getElementById('auth-links').classList.remove('hidden');
        document.getElementById('user-links').classList.add('hidden');
        const usernameDisplay = document.getElementById('user-nickname-display');
        if (usernameDisplay) {
            usernameDisplay.textContent = '';
        }
    }
}