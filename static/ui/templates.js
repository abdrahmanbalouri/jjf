// src/template.js
export const login = `
  <header>
        <div class="container">
            <nav>
                <h1>Forum App</h1>
                <div class="nav-links" id="auth-links">
                    <a href="#" id="nav-login">Login</a>
                    <a href="#" id="nav-register">Register</a>
                </div>
                <div class="nav-links hidden" id="user-links">
                    <span id="user-nickname-display"></span>
                    <a href="#" id="nav-logout">Logout</a>
                </div>
            </nav>
        </div>
    </header>
    <div class="view" id="login-view">
        <h2>Login</h2>
        <div id="login-error" class="error"></div>
        <form id="login-form">
            <div class="form-group">
                <label for="login-identifier">Email or Username</label>
                <input type="text" id="login-identifier" required>
            </div>
            <div class="form-group">
                <label for="login-password">Password</label>
                <input type="password" id="login-password" required>
            </div>
            <button type="submit">Login</button>
        </form>
        <p>Don't have an account? <a href="#" id="show-register">Register</a></p>
    </div>
`;

export const register = `
  <header>
        <div class="container">
            <nav>
                <h1>Forum App</h1>
                <div class="nav-links" id="auth-links">
                    <a href="#" id="nav-login">Login</a>
                    <a href="#" id="nav-register">Register</a>
                </div>
                <div class="nav-links hidden" id="user-links">
                    <span id="user-nickname-display"></span>
                    <a href="#" id="nav-logout">Logout</a>
                </div>
            </nav>
        </div>
    </header>
    <div class="view" id="register-view">
        <h2>Register</h2>
        <div id="register-error" class="error"></div>
        <form id="register-form">
            <div class="form-group">
                <label for="register-nickname">Username</label>
                <input type="text" id="register-nickname" required>
            </div>
            <div class="form-group">
                <label for="register-email">Email</label>
                <input type="email" id="register-email" required>
            </div>
            <div class="form-group">
                <label for="register-password">Password</label>
                <input type="password" id="register-password" required>
            </div>
            <div class="form-group">
                <label for="register-age">Age</label>
                <input type="number" id="register-age" required>
            </div>
            <div class="form-group">
                <label for="register-gender">Gender</label>
                <select id="register-gender" required>
                    <option value="">Select Gender</option>
                    <option value="male">Male</option>
                    <option value="female">Female</option>
                    <option value="other">Other</option>
                </select>
            </div>
            <div class="form-group">
                <label for="register-first-name">First Name</label>
                <input type="text" id="register-first-name" required>
            </div>
            <div class="form-group">
                <label for="register-last-name">Last Name</label>
                <input type="text" id="register-last-name" required>
            </div>
            <button type="submit">Register</button>
        </form>
        <p>Already have an account? <a href="#" id="show-login">Login</a></p>
    </div>
`;

export const posts = `

    <div class="view" id="posts-view">
        <h2>Posts</h2>
        <form id="post-form">
            <div id="post-error" class="error"></div>
            <div class="form-group">
                <label for="post-title">Title</label>
                <input type="text" id="post-title" required>
            </div>
            <div class="form-group">
                <label for="post-category">Category</label>
                <select id="post-category" required>
                    <option value="">Select Category</option>
                    <option value="general">General</option>
                    <option value="tech">Technology</option>
                    <option value="sports">Sports</option>
                </select>
            </div>
            <div class="form-group">
                <label for="post-content">Content</label>
                <textarea id="post-content" required></textarea>
            </div>
            <button type="submit">Create Post</button>
        </form>
        <div id="posts-container"></div>
    </div>
    <div id="comment-popup" class="popup hidden">
        <div class="popup-content">
            <span id="popup-close" class="popup-close">&times;</span>
            <h2 id="popup-post-title" class="text-xl font-bold mb-4">Post Title</h2>
            <div id="popup-comments-container" class="comments-container mb-4"></div>
            <form id="popup-comment-form">
                <div class="form-group">
                    <textarea id="popup-comment-content" class="w-full p-2 border rounded mb-2" placeholder="Write a comment..." required></textarea>
                </div>
                <button type="submit" class="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700">Post Comment</button>
            </form>
        </div>
    </div>
`;

export const messages = `
  <header>
        <div class="container">
            <nav>
                <h1>Forum App</h1>
                <div class="nav-links" id="auth-links">
                    <a href="#" id="nav-login">Login</a>
                    <a href="#" id="nav-register">Register</a>
                </div>
                <div class="nav-links hidden" id="user-links">
                    <span id="user-nickname-display"></span>
                    <a href="#" id="nav-logout">Logout</a>
                </div>
            </nav>
        </div>
    </header>
  <div class="chat-interface">
    <!-- Liste des utilisateurs -->
    <div class="users-panel">
        <h3>Users</h3>
        <div id="typing-indicator" class="typing-indicator"></div>
        <div id="users-list"></div>
    </div>

    <!-- Conversation (cachée initialement) -->
    <div id="conversation-panel" class="hidden">
        <div class="conversation-header">
            <button id="back-to-users" class="back-button">← Retour</button>
            <h2 id="current-chat-user">Discussion avec <span id="receiver-name"></span></h2>
        </div>
        <div class="messages-container" id="messages-container"></div>
        <form id="message-form" class="message-form">
            <div class="form-group">
                <textarea id="message-content" placeholder="Écrivez votre message..." required></textarea>
            </div>
            <button type="submit">Envoyer</button>
        </form>
    </div>
</div>
`;