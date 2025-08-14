// src/managers/PostManager.js
export class PostManager {
    constructor(app) {
        this.app = app;
    }

    setupPostEventListeners() {
        document.getElementById('post-form')?.addEventListener('submit', (e) => this.handlePostCreate(e));
        // Event delegation for comments buttons
        document.getElementById('posts-container')?.addEventListener('click', (e) => {
            if (e.target.classList.contains('view-comments')) {
                const postId = e.target.dataset.postId;
                this.showCommentPopup(postId);
            }
        });
    }

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
                this.loadPosts(); // Reload posts after creation
            } else {
                const error = await response.json();
                if(error.error == "Authentication required" ){
                    throw Error('eror')
                }
                const err = document.getElementById('post-error')
                err.textContent = error.error
                    setTimeout(() => {
                        err.textContent = ""
                    }, 2000)
            }
        } catch (error) {
            this.app.showView('login')
        }
    }

    renderPosts(posts) {
        const container = document.getElementById('posts-container');
        if (!container) return;

        if (!posts || !Array.isArray(posts) || posts.length === 0) {
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
                show

                </button>
            </div>
        `).join('');
        // Event listeners for view-comments buttons are now handled by event delegation in setupPostEventListeners
    }

    async showCommentPopup(postId) {
        try {
            const [postResponse, commentsResponse] = await Promise.all([
                fetch(`/api/posts/${postId}`),
                fetch(`/api/getcomments?post_id=${postId}`)
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
            if (!form) {
                console.error('Popup comment form not found!');
                return;
            }
            // IMPORTANT: Re-apply the event listener cleanup here
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

    async handleCommentCreate(e) {
        e.preventDefault();
        const content = document.getElementById('popup-comment-content').value;
        const postId = e.target.dataset.postId;

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
        if (!container) return;

        container.innerHTML = comments.map(comment => `
            <div class="comment">
                <div class="comment-meta">
                    <span>${comment.author}</span>
                    <span>${new Date(comment.created_at).toLocaleString()}</span>
                </div>
                <div class="comment-content">${comment.content}</div>
            </div>
        `).join('');
        container.scrollTop = container.scrollHeight;


    }

    async loadComments(postId, containerId = 'popup-comments-container') {
        try {
            const response = await fetch(`/api/getcomments?post_id=${postId}`);
            if (!response.ok) throw new Error('Failed to load comments');
            const comments = await response.json();
            this.renderComments(comments, containerId);
        } catch (error) {
            console.error('Error loading comments:', error);
            document.getElementById(containerId).innerHTML =
                '<div class="error">Failed to load comments</div>';
        }
    }
}