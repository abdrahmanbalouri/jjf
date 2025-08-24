// src/managers/PostManager.js
export class PostManager {
    constructor(app) {
        this.app = app;
        this.offsetpost = 0
        this.page = false
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
        setInterval(() => {
            if (window.innerWidth > 500) {

                let conv = document.getElementById('conversation-panel')
                conv.style.maxWidth = ''
                if (document.getElementById('users-list').style.display == 'none'
                ) {
                    document.getElementById('users-list').style.display = 'block'

                }

            }
        }, 100)
        document.querySelector('.users-panel')?.addEventListener('click', (e) => {
            if (e.target !== e.currentTarget) return;

            if (window.innerWidth <= 500) {
                if (this.page === false) {
                    document.getElementById('users-list').style.display = 'block'

                    let conv = document.getElementById('conversation-panel')
                    conv.style.maxWidth = '200px'
                    let use = document.querySelector('.users-panel');
                    use.style.width = 'auto';
                    use.style.height = 'auto';
                    use.style.borderRadius = '0px';



                    use.style.background = 'linear-gradient(185deg, #7851ef 50%, #e0e7ff 100%)';
                    use.style.padding = '20px';
                    use.style.borderRight = '1px solid var(--border-color)';
                    use.style.overflowY = 'auto';
                    use.style.transition = 'var(--transition)';

                    this.page = true;
                } else {
                    document.getElementById('users-list').style.display = 'none'
                    document.getElementById('conversation-panel').classList.add('hidden');

                    this.app.currentConversation = null;
                    document.getElementById('message-content').value = '';
                    let use = document.querySelector('.users-panel');
                    use.style.borderRadius = '';
                    use.style.height = ''
                    use.style.width = ''
                    use.style.background = '';
                    use.style.padding = '';
                    use.style.borderRight = '';
                    use.style.overflowY = '';
                    use.style.transition = '';

                    this.page = false;

                }
            }


        })

    }


    async loadPosts() {
        try {
            const response = await fetch(`/api/posts?with=${this.offsetpost}`);

            if (!response.ok) throw new Error('Failed to load posts');
            const posts = await response.json();
            this.renderPosts(posts || []);
            const messagesContainer = document.getElementById('posts-container');
            if (messagesContainer) {
                //   messagesContainer.scrollTop = messagesContainer.scrollHeight;
                // Remove any existing scroll listeners to prevent duplicates
                messagesContainer.onscroll = null;


                messagesContainer.onscroll = this.throttle(() => {


                    const scrollTop = messagesContainer.scrollTop;
                    const scrollHeight = messagesContainer.scrollHeight;
                    const clientHeight = messagesContainer.clientHeight;
                    console.log(scrollTop, scrollHeight, clientHeight);

                    console.log(scrollTop + clientHeight >= scrollHeight - 50);


                    if (scrollTop + clientHeight >= scrollHeight - 10) {
                        this.loadpost();
                    }


                }, 500);
            }

        } catch (error) {
            console.error('Error loading posts:', error);
            document.getElementById('posts-container').innerHTML =
                '<div class="error">Failed to load posts. Please try again later.</div>';
        }
    }
    async loadpostforcreate() {
        try {
            const response = await fetch(`/api/posts/forcreate`);

            if (!response.ok) throw new Error('Failed to load posts');
            const posts = await response.json();
            this.renderPostsfor(posts || []);


        } catch (error) {
            console.error('Error loading posts:', error);
            document.getElementById('posts-container').innerHTML =
                '<div class="error">Failed to load posts. Please try again later.</div>';
        }
    }
    async loadpost() {



        await this.loadPosts();


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
    async handlePostCreate(e) {

        e.preventDefault();
        const token = this.getCookie('session_id');
        try {

            const response = await fetch(`/api/auto?with=${token}`);
            if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();


            if (id !== this.app.currentUser.id) {
                if (this.app.socket) {
                    this.app.socket.close();
                }
                this.app.authManager.handleLogout()
                return
            }
        } catch (err) {
            console.log(err);

        }
        const title = document.getElementById('post-title').value;
        const content = document.getElementById('post-content').value;
        const category = document.getElementById('post-category').value;

        if (!title || !content || !category) {
            const err = document.getElementById('post-error')
            err.textContent = "All fields are required"
            setTimeout(() => {
                err.textContent = ""
            }, 2000)
            return;

        }
        //   console.log(category);

        if (category != 'general ' && category != 'tech' && category != "sports") {
            const err = document.getElementById('post-error')
            err.textContent = "no inspects plzzz"
            setTimeout(() => {
                err.textContent = ""
            }, 2000)
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
                this.loadpostforcreate(); // Reload posts after creation
            } else {
                const error = await response.json();
                if (error.error == "Authentication required") {
                    console.log(' dfgdfgd');

                    throw Error('eror')
                }
                const err = document.getElementById('post-error')
                err.textContent = error.error
                setTimeout(() => {
                    err.textContent = ""
                }, 2000)
            }
        } catch (error) {
            console.log(error);


            if (error.message === 'eror') {
                this.app.authManager.handleLogout()
            }
        }
    }
    getCookie(name) {
        return document.cookie
            .split('; ')
            .find(row => row.startsWith(name + '='))
            ?.split('=')[1] || null;
    }

    renderPosts(posts) {
        const container = document.getElementById('posts-container');
        if (!container) return;

        if (posts.length == 0) {
            return;
        }
        console.log(posts);

        this.offsetpost += 10;


        //   const scrollTop = container.scrollTop;
        const clientHeight = container.clientHeight;
        container.scrollTop = container.scrollHeight - clientHeight



        const postss = posts.map(post => `
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
        container.insertAdjacentHTML('beforeend', postss);

        // Event listeners for view-comments buttons are now handled by event delegation in setupPostEventListeners
    }
    renderPostsfor(posts) {
        const container = document.getElementById('posts-container');
        if (!container) return;

        if (posts.length == 0) {
            return;
        }
        console.log(posts, '----------------');

        this.offsetpost++;



        console.log(posts);


        const postss = `
            <div class="post" data-id="${posts.id}">
                <h3 class="post-title">${posts.title}</h3>
                <div class="post-meta">
                    <span>Posted by ${posts.author || 'Unknown'} in ${posts.category || 'General'}</span>
                    <span>${posts.created_at ? new Date(posts.created_at).toLocaleString() : ''}</span>
                </div>
                <div class="post-content">${posts.content || ''}</div>
                <button class="view-comments" data-post-id="${posts.id}">
                show

                </button>
            </div>
        `
        container.scrollTop = 0
        container.insertAdjacentHTML('afterbegin', postss);

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
        const token = this.getCookie('session_id');
        try {

            const response = await fetch(`/api/auto?with=${token}`);
            if (!response.ok) throw new Error('Failed to load users');
            const id = await response.json();


            if (id !== this.app.currentUser.id) {
                if (this.app.socket) {
                    this.app.socket.close();
                }
                this.app.authManager.handleLogout()
                return
            }
        } catch (err) {
            console.log(err);

        }
        const content = document.getElementById('popup-comment-content').value;
        const postId = e.target.dataset.postId;



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
                const err = document.getElementById('comment-error')
                if (error.error == "Authentication required") {
                    throw Error('eror')
                }
                err.textContent = error.error
                setTimeout(() => {
                    err.textContent = ""
                }, 2000)
            }
        } catch (error) {
            if (error.message === 'eror') {
                this.app.authManager.handleLogout()
            }

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