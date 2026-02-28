/* ============================================
   BRUTALIST BLOG - DYNAMIC INTERACTIONS
   ============================================ */

// ==================== PAGINATION VARIABLES ====================
let currentPage = 1;
const postsPerPage = 6;
let totalPages = 1;
let currentFilter = 'all';

// ==================== PERFORMANCE FLAGS ====================
let isInitialized = false;
let scrollObserver = null;
let cachedFilteredPosts = null;
let lastFilter = null;

// ==================== DEBOUNCE UTILITY ====================
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// ==================== THROTTLE UTILITY ====================
function throttle(func, limit) {
    let inThrottle;
    return function(...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

// ==================== THEME TOGGLE ====================
function initThemeToggle() {
    const themeToggle = document.getElementById('themeToggle');
    if (!themeToggle) return;
    
    // Check for saved theme preference or default to 'light'
    const currentTheme = localStorage.getItem('theme') || 'light';
    document.documentElement.setAttribute('data-theme', currentTheme);
    
    // Toggle theme
    themeToggle.addEventListener('click', () => {
        const theme = document.documentElement.getAttribute('data-theme');
        const newTheme = theme === 'light' ? 'dark' : 'light';
        
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        
        // Add animation effect
        themeToggle.style.transform = 'rotate(360deg)';
        setTimeout(() => {
            themeToggle.style.transform = 'rotate(0deg)';
        }, 300);
    });
}

// ==================== CUSTOM CURSOR ====================
function initCustomCursor() {
    if (window.matchMedia('(pointer: coarse)').matches) return; // Skip on touch devices
    
    const cursorDot = document.querySelector('.cursor-dot');
    const cursorOutline = document.querySelector('.cursor-outline');
    
    if (!cursorDot || !cursorOutline) return;
    
    let mouseX = 0;
    let mouseY = 0;
    let outlineX = 0;
    let outlineY = 0;
    
    document.addEventListener('mousemove', (e) => {
        mouseX = e.clientX;
        mouseY = e.clientY;
        
        cursorDot.style.left = mouseX + 'px';
        cursorDot.style.top = mouseY + 'px';
    }, { passive: true });
    
    // Smooth follow for outline
    function animateCursor() {
        const distX = mouseX - outlineX;
        const distY = mouseY - outlineY;
        
        outlineX += distX * 0.15;
        outlineY += distY * 0.15;
        
        cursorOutline.style.left = outlineX + 'px';
        cursorOutline.style.top = outlineY + 'px';
        
        requestAnimationFrame(animateCursor);
    }
    animateCursor();
    
    // Event delegation for cursor interactions
    document.body.addEventListener('mouseenter', (e) => {
        if (e.target.matches('a, button, .blog-card, .brutal-filter, .page-number')) {
            cursorDot.style.transform = 'scale(2)';
            cursorOutline.style.transform = 'scale(1.5)';
        }
    }, true);
    
    document.body.addEventListener('mouseleave', (e) => {
        if (e.target.matches('a, button, .blog-card, .brutal-filter, .page-number')) {
            cursorDot.style.transform = 'scale(1)';
            cursorOutline.style.transform = 'scale(1)';
        }
    }, true);
}

// ==================== SCROLL ANIMATIONS ====================
function initScrollAnimations() {
    if (scrollObserver) {
        // Observe new elements only
        const newElements = document.querySelectorAll('.scroll-animate:not(.observed)');
        newElements.forEach(el => {
            el.classList.add('observed');
            scrollObserver.observe(el);
        });
        return;
    }
    
    const observerOptions = {
        threshold: 0.1,
        rootMargin: '0px 0px -50px 0px'
    };
    
    scrollObserver = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                entry.target.classList.add('show');
                scrollObserver.unobserve(entry.target); // Stop observing once shown
            }
        });
    }, observerOptions);
    
    const animateElements = document.querySelectorAll('.scroll-animate');
    animateElements.forEach(el => {
        el.classList.add('observed');
        scrollObserver.observe(el);
    });
}

// ==================== HEADER SCROLL EFFECT ====================
function initHeaderScroll() {
    const header = document.querySelector('.brutal-header');
    let lastScroll = 0;
    let ticking = false;
    
    const updateHeader = () => {
        const currentScroll = window.pageYOffset;
        
        if (currentScroll > 100) {
            header.style.boxShadow = '0 4px 20px rgba(0,0,0,0.1)';
        } else {
            header.style.boxShadow = 'none';
        }
        
        lastScroll = currentScroll;
        ticking = false;
    };
    
    window.addEventListener('scroll', () => {
        if (!ticking) {
            window.requestAnimationFrame(updateHeader);
            ticking = true;
        }
    }, { passive: true });
}

// ==================== BLOG DISPLAY & FILTERING ====================
function displayPosts(filter = 'all') {
    if (typeof blogPosts === 'undefined') {
        console.error('Blog posts data not loaded');
        return;
    }
    
    currentFilter = filter;
    
    // Use cached filtered posts if filter hasn't changed
    let filteredPosts;
    if (filter === lastFilter && cachedFilteredPosts) {
        filteredPosts = cachedFilteredPosts;
    } else {
        if (filter !== 'all') {
            filteredPosts = blogPosts.filter(post => 
                post.tag.toLowerCase().includes(filter.toLowerCase())
            );
        } else {
            filteredPosts = blogPosts;
        }
        cachedFilteredPosts = filteredPosts;
        lastFilter = filter;
    }
    
    // Calculate pagination
    totalPages = Math.ceil(filteredPosts.length / postsPerPage);
    const startIndex = (currentPage - 1) * postsPerPage;
    const endIndex = startIndex + postsPerPage;
    const postsToShow = filteredPosts.slice(startIndex, endIndex);
    
    const blogGrid = document.getElementById('blogGrid');
    if (!blogGrid) return;
    
    // Use DocumentFragment for better performance
    const fragment = document.createDocumentFragment();
    
    postsToShow.forEach((post, index) => {
        const card = createBlogCard(post, index);
        fragment.appendChild(card);
    });
    
    // Clear and append all at once
    blogGrid.innerHTML = '';
    blogGrid.appendChild(fragment);
    
    // Initialize animations for new elements after paint
    requestAnimationFrame(() => {
        requestAnimationFrame(() => {
            initScrollAnimations();
        });
    });
    
    updatePagination(filteredPosts.length);
}

// ==================== BLOG CARD CREATION ====================
function createBlogCard(post, index) {
    const card = document.createElement('article');
    card.className = 'blog-card scroll-animate';
    card.style.animationDelay = `${index * 0.05}s`;
    
    const imageWrapper = document.createElement('div');
    imageWrapper.className = 'card-image-wrapper';
    
    const img = document.createElement('img');
    img.src = post.thumbnail || 'images/placeholder.jpg';
    img.alt = post.title;
    img.className = 'card-image';
    img.loading = 'lazy';
    img.decoding = 'async';
    // Add placeholder background while loading
    img.style.backgroundColor = '#1a1a1a';
    
    const tag = document.createElement('div');
    tag.className = 'card-tag';
    tag.textContent = post.tag.split(',')[0].trim();
    
    imageWrapper.appendChild(img);
    imageWrapper.appendChild(tag);
    
    const content = document.createElement('div');
    content.className = 'card-content';
    
    const date = document.createElement('div');
    date.className = 'card-date';
    date.textContent = post.date;
    
    const title = document.createElement('h3');
    title.className = 'card-title';
    title.textContent = post.title;
    
    const summary = document.createElement('p');
    summary.className = 'card-summary';
    summary.textContent = truncateText(post.summary, 120);
    
    const link = document.createElement('a');
    link.href = post.link;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.className = 'card-link';
    link.innerHTML = '<span>READ MORE</span><span class="card-arrow">→</span>';
    
    content.appendChild(date);
    content.appendChild(title);
    content.appendChild(summary);
    content.appendChild(link);
    
    card.appendChild(imageWrapper);
    card.appendChild(content);
    
    return card;
}

function truncateText(text, maxLength) {
    if (text.length <= maxLength) return text;
    return text.substr(0, maxLength) + '...';
}

// ==================== PAGINATION ====================
function updatePagination(totalPosts) {
    const pageNumbers = document.getElementById('pageNumbers');
    const prevBtn = document.getElementById('prevBtn');
    const nextBtn = document.getElementById('nextBtn');
    const pageInfo = document.getElementById('pageInfo');
    
    if (!pageNumbers) return;
    
    pageNumbers.innerHTML = '';
    
    // Show page numbers
    const maxVisiblePages = 5;
    let startPage = Math.max(1, currentPage - Math.floor(maxVisiblePages / 2));
    let endPage = Math.min(totalPages, startPage + maxVisiblePages - 1);
    
    if (endPage - startPage < maxVisiblePages - 1) {
        startPage = Math.max(1, endPage - maxVisiblePages + 1);
    }
    
    for (let i = startPage; i <= endPage; i++) {
        const pageBtn = document.createElement('button');
        pageBtn.textContent = i;
        pageBtn.className = i === currentPage ? 'page-number active' : 'page-number';
        pageBtn.onclick = () => goToPage(i);
        pageNumbers.appendChild(pageBtn);
    }
    
    // Update buttons
    if (prevBtn) prevBtn.disabled = currentPage === 1;
    if (nextBtn) nextBtn.disabled = currentPage === totalPages;
    
    // Update info
    if (pageInfo) {
        const startPost = (currentPage - 1) * postsPerPage + 1;
        const endPost = Math.min(currentPage * postsPerPage, totalPosts);
        pageInfo.textContent = `SHOWING ${startPost}-${endPost} OF ${totalPosts}`;
    }
}

function changePage(direction) {
    const newPage = currentPage + direction;
    if (newPage >= 1 && newPage <= totalPages) {
        currentPage = newPage;
        displayPosts(currentFilter);
        scrollToBlog();
    }
}

function goToPage(pageNumber) {
    currentPage = pageNumber;
    displayPosts(currentFilter);
    scrollToBlog();
}

function scrollToBlog() {
    const blogSection = document.getElementById('blog');
    if (blogSection) {
        blogSection.scrollIntoView({
            behavior: 'smooth',
            block: 'start'
        });
    }
}

// ==================== FILTER SYSTEM ====================
function initFilterSystem() {
    const filterButtons = document.querySelectorAll('.brutal-filter');
    
    filterButtons.forEach(button => {
        button.addEventListener('click', () => {
            // Update active state
            filterButtons.forEach(btn => btn.classList.remove('active'));
            button.classList.add('active');
            
            // Get filter value
            const filter = button.getAttribute('data-filter');
            currentPage = 1; // Reset to page 1
            
            // Clear cache when filter changes
            cachedFilteredPosts = null;
            lastFilter = null;
            
            displayPosts(filter);
        });
    });
}

// ==================== SMOOTH SCROLL ====================
function initSmoothScroll() {
    const links = document.querySelectorAll('a[href^="#"]');
    
    links.forEach(link => {
        link.addEventListener('click', function(e) {
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                e.preventDefault();
                target.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });
}

// ==================== NAV ACTIVE STATE ====================
function initNavActive() {
    const sections = document.querySelectorAll('section[id]');
    const navLinks = document.querySelectorAll('.nav-link');
    
    const updateActiveNav = throttle(() => {
        let current = '';
        
        sections.forEach(section => {
            const sectionTop = section.offsetTop;
            
            if (window.pageYOffset >= sectionTop - 200) {
                current = section.getAttribute('id');
            }
        });
        
        navLinks.forEach(link => {
            link.classList.remove('active');
            if (link.getAttribute('href') === `#${current}`) {
                link.classList.add('active');
            }
        });
    }, 100);
    
    window.addEventListener('scroll', updateActiveNav, { passive: true });
    updateActiveNav();
}

// ==================== ANIMATE STATS ====================
function animateStats() {
    const postCount = document.getElementById('postCount');
    if (!postCount || typeof blogPosts === 'undefined') return;
    
    const target = blogPosts.length;
    let current = 0;
    const increment = target / 50;
    
    const timer = setInterval(() => {
        current += increment;
        if (current >= target) {
            postCount.textContent = target;
            clearInterval(timer);
        } else {
            postCount.textContent = Math.floor(current);
        }
    }, 30);
}

// ==================== HERO WORD ANIMATIONS ====================
function initHeroAnimations() {
    const words = document.querySelectorAll('.word');
    
    words.forEach((word, index) => {
        word.style.setProperty('--word-index', index);
    });
}

// ==================== MOBILE MENU ====================
function initMobileMenu() {
    const burger = document.querySelector('.menu-burger');
    const navLinks = document.querySelector('.nav-links');
    const navItems = document.querySelectorAll('.nav-link');
    
    if (burger && navLinks) {
        burger.addEventListener('click', () => {
            navLinks.classList.toggle('active');
            burger.classList.toggle('active');
            document.body.style.overflow = navLinks.classList.contains('active') ? 'hidden' : '';
        });
        
        // Close menu when clicking a link
        navItems.forEach(link => {
            link.addEventListener('click', () => {
                navLinks.classList.remove('active');
                burger.classList.remove('active');
                document.body.style.overflow = '';
            });
        });
        
        // Close menu when clicking outside
        document.addEventListener('click', (e) => {
            if (navLinks.classList.contains('active') && 
                !navLinks.contains(e.target) && 
                !burger.contains(e.target)) {
                navLinks.classList.remove('active');
                burger.classList.remove('active');
                document.body.style.overflow = '';
            }
        });
    }
}

// ==================== PARALLAX ELEMENTS ====================
function initParallax() {
    // Disabled for performance - CSS animations are sufficient
    return;
}

// ==================== LOADING ANIMATION ====================
function initPageLoad() {
    document.body.classList.add('loading');
    
    window.addEventListener('load', () => {
        setTimeout(() => {
            document.body.classList.remove('loading');
            document.body.classList.add('loaded');
        }, 300);
    });
}

// ==================== INITIALIZE ALL ====================
document.addEventListener('DOMContentLoaded', function() {
    console.log('🎨 Initializing Brutalist Blog...');
    
    // Initialize theme first
    initThemeToggle();
    
    // Initialize core features
    initPageLoad();
    initCustomCursor();
    initScrollAnimations();
    initHeaderScroll();
    initSmoothScroll();
    initNavActive();
    initFilterSystem();
    initHeroAnimations();
    initMobileMenu();
    initParallax();
    
    // Initialize blog data
    if (typeof blogPosts !== 'undefined') {
        totalPages = Math.ceil(blogPosts.length / postsPerPage);
        displayPosts('all');
        animateStats();
        console.log(`✅ Loaded ${blogPosts.length} posts`);
    } else {
        console.warn('⚠️ Blog posts not loaded');
    }
    
    console.log('✨ Blog initialized successfully!');
});

// ==================== WINDOW RESIZE ====================
window.addEventListener('resize', debounce(() => {
    if (currentFilter) {
        displayPosts(currentFilter);
    }
}, 250));

// ==================== KEYBOARD NAVIGATION ====================
document.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowLeft' && currentPage > 1) {
        changePage(-1);
    } else if (e.key === 'ArrowRight' && currentPage < totalPages) {
        changePage(1);
    }
});