// Pagination variables
let currentPage = 1;
const postsPerPage = 4;
let totalPages = 1; // Will be calculated after blog data loads

// Three.js background animation
let scene, camera, renderer, particles;

function initThree() {
    scene = new THREE.Scene();
    camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
    renderer = new THREE.WebGLRenderer({ 
        alpha: true,
        antialias: true,
        premultipliedAlpha: false // Helps prevent white flash
    });
    
    // Set clear color to match your background
    renderer.setClearColor(0x0f0f0f, 0); // Dark background, fully transparent
    
    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.domElement.style.position = 'fixed';
    renderer.domElement.style.top = '0';
    renderer.domElement.style.left = '0';
    renderer.domElement.style.zIndex = '-1';
    renderer.domElement.style.pointerEvents = 'none';
    
    // Start with canvas hidden
    renderer.domElement.style.opacity = '0';
    
    document.querySelector('.hero-bg').appendChild(renderer.domElement);

    // Create particles
    const geometry = new THREE.BufferGeometry();
    const particleCount = 10000;
    const positions = new Float32Array(particleCount * 3);
    const colors = new Float32Array(particleCount * 3);

    for (let i = 0; i < particleCount * 3; i += 3) {
        positions[i] = (Math.random() - 0.5) * 20;
        positions[i + 1] = (Math.random() - 0.5) * 20;
        positions[i + 2] = (Math.random() - 0.5) * 20;

        colors[i] = Math.random() * 0.5 + 0.5;
        colors[i + 1] = Math.random() * 0.3 + 0.7;
        colors[i + 2] = 1;
    }

    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));

    const material = new THREE.PointsMaterial({
        size: 0.02,
        vertexColors: true,
        transparent: true,
        opacity: 0.8
    });

    particles = new THREE.Points(geometry, material);
    scene.add(particles);

    camera.position.z = 5;
    
    // Fade in canvas after a short delay
    setTimeout(() => {
        renderer.domElement.style.transition = 'opacity 0.5s ease-in-out';
        renderer.domElement.style.opacity = '1';
        renderer.domElement.classList.add('ready');
    }, 100);
}

function animateThree() {
    requestAnimationFrame(animateThree);
    
    if (particles) {
        particles.rotation.x += 0.0005;
        particles.rotation.y += 0.001;
    }

    renderer.render(scene, camera);
}

// Pagination functions
function displayPosts() {
    // Check if blogPosts is available (from blog.js)
    if (typeof blogPosts === 'undefined') {
        console.error('Blog posts data not loaded. Make sure blog.js is included before script.js');
        return;
    }

    const blogGrid = document.getElementById('blogGrid');
    const startIndex = (currentPage - 1) * postsPerPage;
    const endIndex = startIndex + postsPerPage;
    const postsToShow = blogPosts.slice(startIndex, endIndex);

    blogGrid.innerHTML = '';

    postsToShow.forEach(post => {
        const blogCard = document.createElement('article');
        blogCard.className = 'blog-card scroll-animate';
        
        // Create thumbnail HTML if thumbnail exists
        const thumbnailHTML = post.thumbnail ? 
            `<div class="blog-thumbnail">
                <img src="${post.thumbnail}" alt="${post.title}" loading="lazy">
            </div>` : '';

        blogCard.innerHTML = `
            ${thumbnailHTML}
            <div class="blog-content">
                <div class="blog-meta">
                    <span class="blog-date">${post.date}</span>
                    <span class="blog-tag">${post.tag}</span>
                </div>
                <h3>${post.title}</h3>
                <p class="blog-summary">${post.summary}</p>
                <a href="${post.link}" target="_blank" class="read-more">Read Full Blog</a>
            </div>
        `;
        blogGrid.appendChild(blogCard);
    });

    // Trigger scroll animation for new posts
    setTimeout(() => {
        handleScroll();
        // RE-INITIALIZE HOVER EFFECTS FOR NEW CARDS
        initCardEffects();
    }, 100);
}

function updatePagination() {
    // Check if blogPosts is available
    if (typeof blogPosts === 'undefined') {
        return;
    }

    // Recalculate total pages in case blog data changed
    totalPages = Math.ceil(blogPosts.length / postsPerPage);

    const pageNumbers = document.getElementById('pageNumbers');
    const prevBtn = document.getElementById('prevBtn');
    const nextBtn = document.getElementById('nextBtn');
    const pageInfo = document.getElementById('pageInfo');

    // Clear existing page numbers
    pageNumbers.innerHTML = '';

    // Create page number buttons
    for (let i = 1; i <= totalPages; i++) {
        const pageBtn = document.createElement('button');
        pageBtn.textContent = i;
        pageBtn.className = i === currentPage ? 'active' : '';
        pageBtn.onclick = () => goToPage(i);
        pageNumbers.appendChild(pageBtn);
    }

    // Update navigation buttons
    prevBtn.disabled = currentPage === 1;
    nextBtn.disabled = currentPage === totalPages;

    // Update page info
    const startPost = (currentPage - 1) * postsPerPage + 1;
    const endPost = Math.min(currentPage * postsPerPage, blogPosts.length);
    pageInfo.textContent = `Showing ${startPost}-${endPost} of ${blogPosts.length} posts`;
}

function changePage(direction) {
    const newPage = currentPage + direction;
    if (newPage >= 1 && newPage <= totalPages) {
        currentPage = newPage;
        displayPosts();
        updatePagination();
        
        // Smooth scroll to blog section
        document.getElementById('blog').scrollIntoView({
            behavior: 'smooth',
            block: 'start'
        });
    }
}

function goToPage(pageNumber) {
    currentPage = pageNumber;
    displayPosts();
    updatePagination();
    
    // Smooth scroll to blog section
    document.getElementById('blog').scrollIntoView({
        behavior: 'smooth',
        block: 'start'
    });
}

// Blog filtering functions (using BlogData helper)
function filterByTag(tag) {
    if (typeof BlogData !== 'undefined') {
        const filteredPosts = BlogData.getPostsByTag(tag);
        // You can implement custom filtering UI here
        console.log(`Posts tagged with "${tag}":`, filteredPosts);
    }
}

function searchPosts(query) {
    if (typeof BlogData !== 'undefined') {
        const searchResults = BlogData.searchPosts(query);
        // You can implement search results UI here
        console.log(`Search results for "${query}":`, searchResults);
    }
}

// Scroll animations
function handleScroll() {
    const elements = document.querySelectorAll('.scroll-animate');
    elements.forEach(element => {
        const elementTop = element.getBoundingClientRect().top;
        const elementVisible = 150;
        
        if (elementTop < window.innerHeight - elementVisible) {
            element.classList.add('visible');
        }
    });
}

// Header background on scroll
function handleHeaderScroll() {
    const header = document.querySelector('header');
    if (window.scrollY > 100) {
        header.style.background = 'rgba(15, 15, 15, 0.95)';
    } else {
        header.style.background = 'rgba(15, 15, 15, 0.8)';
    }
}

// Smooth scrolling for navigation links
function smoothScroll() {
    const links = document.querySelectorAll('a[href^="#"]');
    links.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });
}

// Parallax effect for hero section
function handleParallax() {
    const scrolled = window.pageYOffset;
    const hero = document.querySelector('.hero-content');
    if (hero) {
        hero.style.transform = `translateY(${scrolled * 0.5}px)`;
    }
}

// Card hover effects
function initCardEffects() {
    const cards = document.querySelectorAll('.blog-card');
    cards.forEach(card => {
        card.addEventListener('mouseenter', function() {
            this.style.transform = 'translateY(-8px) scale(1.02)';
        });
        
        card.addEventListener('mouseleave', function() {
            this.style.transform = 'translateY(0) scale(1)';
        });
    });
}

// Initialize everything
document.addEventListener('DOMContentLoaded', function() {
    // Calculate total pages once blog data is available
    if (typeof blogPosts !== 'undefined') {
        totalPages = Math.ceil(blogPosts.length / postsPerPage);
    }

    initThree();
    animateThree();
    smoothScroll();
    handleScroll(); // Initial check
    
    // Initialize pagination
    displayPosts();
    updatePagination();
    
    // Re-initialize card effects after posts are loaded
    setTimeout(() => {
        initCardEffects();
    }, 200);
    
    window.addEventListener('scroll', function() {
        handleScroll();
        handleHeaderScroll();
        handleParallax();
    });
    
    window.addEventListener('resize', function() {
        camera.aspect = window.innerWidth / window.innerHeight;
        camera.updateProjectionMatrix();
        renderer.setSize(window.innerWidth, window.innerHeight);
    });
});

document.addEventListener('DOMContentLoaded', function() {
    // Ensure body is visible after everything loads
    setTimeout(() => {
        document.body.style.opacity = '1';
    }, 50);
});