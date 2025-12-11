// Auth page functionality
document.addEventListener('DOMContentLoaded', function() {
    const signinForm = document.getElementById('signinForm');
    const signupForm = document.getElementById('signupForm');
    const showSignupBtn = document.getElementById('showSignup');
    const showSigninBtn = document.getElementById('showSignin');
    const loginForm = document.getElementById('loginForm');
    const registerForm = document.getElementById('registerForm');

    // Toggle between sign in and sign up forms
    if (showSignupBtn) {
        showSignupBtn.addEventListener('click', function(e) {
            e.preventDefault();
            signinForm.classList.remove('active');
            signupForm.classList.add('active');
        });
    }

    if (showSigninBtn) {
        showSigninBtn.addEventListener('click', function(e) {
            e.preventDefault();
            signupForm.classList.remove('active');
            signinForm.classList.add('active');
        });
    }

    // Handle login form submission
    if (loginForm) {
        loginForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const identifier = document.getElementById('loginEmail').value; // can be email or username
            const password = document.getElementById('loginPassword').value;

            console.log('Attempting login with identifier:', identifier);

            try {
                const response = await fetch('/api/auth/login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ identifier, password })
                });

                const data = await response.json();

                if (response.ok) {
                    console.log('Login successful:', data);
                    
                    // Store token
                    localStorage.setItem('token', data.token);
                    localStorage.setItem('user', JSON.stringify(data.user));
                    
                    // Show success notification
                    showNotification('Login successful!', 'success');
                    
                    // Redirect to home page
                    setTimeout(() => {
                        window.location.href = '/';
                    }, 1000);
                } else {
                    const errorMsg = data.error || 'Login failed';
                    console.error('Login failed:', errorMsg, data);
                    showNotification(errorMsg, 'error');
                }
            } catch (error) {
                console.error('Login error:', error);
                showNotification('An error occurred during login. Check console for details.', 'error');
            }
        });
    }

    // Handle register form submission
    if (registerForm) {
        registerForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const username = document.getElementById('registerUsername').value;
            const email = document.getElementById('registerEmail').value;
            const password = document.getElementById('registerPassword').value;
            const passwordConfirm = document.getElementById('registerPasswordConfirm').value;

            // Validate passwords match
            if (password !== passwordConfirm) {
                console.error('Password mismatch');
                showNotification('Passwords do not match', 'error');
                return;
            }

            console.log('Attempting registration with username:', username, 'email:', email);

            try {
                const response = await fetch('/api/auth/register', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ username, email, password })
                });

                const data = await response.json();

                if (response.ok) {
                    console.log('Registration successful:', data);
                    
                    // Store token
                    localStorage.setItem('token', data.token);
                    localStorage.setItem('user', JSON.stringify(data.user));
                    
                    // Show success notification
                    showNotification('Account created successfully!', 'success');
                    
                    // Redirect to home page
                    setTimeout(() => {
                        window.location.href = '/';
                    }, 1000);
                } else {
                    const errorMsg = data.error || 'Registration failed';
                    console.error('Registration failed:', errorMsg, data);
                    showNotification(errorMsg, 'error');
                }
            } catch (error) {
                console.error('Registration error:', error);
                showNotification('An error occurred during registration. Check console for details.', 'error');
            }
        });
    }

    // Notification function with top-right positioning
    function showNotification(message, type) {
        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.textContent = message;
        document.body.appendChild(notification);

        // Auto-remove after 4 seconds
        setTimeout(() => {
            notification.style.opacity = '0';
            setTimeout(() => {
                notification.remove();
            }, 300);
        }, 4000);
    }
});
